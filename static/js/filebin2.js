function FileAPI (c, t, d, f, bin, binURL) {

    var fileCount = c,
        fileList = t,
        dropZone = d,
        fileField = f,
        counter_queue = 0,
        counter_uploading = 0,
        counter_completed = 0,
        counter_failed = 0,
        concurrency = 4,
        fileQueue = [],
        queueProcessorRunning = false;

    function showDropZone() {
        dropZone.style.visibility = "visible";
    }
    function hideDropZone() {
        dropZone.style.visibility = "hidden";
    }
    function allowDrag(e) {
        e.dataTransfer.dropEffect = 'copy';
        e.preventDefault();
    }

    // https://stackoverflow.com/questions/19841859/full-page-drag-and-drop-files-website
    window.addEventListener('dragenter', function(e) {
        showDropZone();
    });

    window.addEventListener('dragover', function(e) {
        showDropZone();
    });

    this.init = function () {
        if (fileField) {
            fileField.onchange = this.addFiles;
        }
        dropZone.addEventListener("dragenter",  this.stopProp, allowDrag);
        dropZone.addEventListener("dragleave",  this.dragExit, false);
        dropZone.addEventListener("dragover",  this.dragOver, allowDrag);
        dropZone.addEventListener("drop",  this.showDroppedFiles);
    }

    this.addFiles = function () {
        addFileListItems(this.files);
    }

    function updateFileCount() {
        var box = document.getElementById('fileCount');

        // TODO: Make this less messy
        var text = counter_completed + " of " + counter_queue + " file";
        if (counter_queue !== 1){
            text = text + "s";
        }
        text = text + " uploaded";
        if (counter_failed > 0) {
            text = text + ". " + counter_failed + " failed.";
            box.className = "alert alert-danger";
        } else if (counter_completed === counter_queue) {
            text = text + ", all done!";
            box.className = "alert alert-success";

            // Automatic refresh when uploads complete
            //window.location.assign(binURL)
            window.location.href = binURL;
        }

        if ((counter_completed + counter_failed) !== counter_queue) {
            text = text + "... please wait.";
            box.className = "alert alert-info";
        }

        fileCount.textContent = text;
        box.style.display = 'block';
    }

    this.showDroppedFiles = function (ev) {
        ev.stopPropagation();
        ev.preventDefault();
        hideDropZone();
        var files = ev.dataTransfer.files;
        addFileListItems(files);
    }

    this.dragOver = function (ev) {
        ev.stopPropagation();
        ev.preventDefault();
        showDropZone();
    }

    this.dragExit = function (ev) {
        ev.stopPropagation();
        ev.preventDefault();
        hideDropZone();
    }

    this.stopProp = function (ev) {
        ev.stopPropagation();
        ev.preventDefault();
        showDropZone();
    }

    // Process the upload queue with concurrency control
    var processQueue = function () {
        // Don't start another processor if one is already running
        if (queueProcessorRunning) {
            return;
        }
        queueProcessorRunning = true;

        var processUploads = function () {
            // Initiate uploads up to the concurrency limit
            while (counter_uploading < concurrency && fileQueue.length > 0) {
                var item = fileQueue.pop();
                uploadFile(item.file, item.container);
            }
        };

        // Process immediately
        processUploads();

        // Continue checking every 100ms until queue is empty
        var i = setInterval(function(){
            processUploads();

            // Break out of the loop when the queue is empty and all uploads done
            if (fileQueue.length === 0 && counter_uploading === 0) {
                clearInterval(i);
                queueProcessorRunning = false;
            }
        }, 100);
    };

    // Legacy event handler for backwards compatibility
    this.uploadQueue = function (ev) {
        if (ev) {
            ev.preventDefault();
        }
        // Defer to next event loop tick to ensure files are queued first
        setTimeout(processQueue, 0);
    };

    var addFileListItems = function (files) {
        counter_queue += files.length;
        updateFileCount();
        for (var i = 0; i < files.length; i++) {
            showFileInList(files[i]);
        }
    }

    var showFileInList = function (file) {
        if (file) {
            var container = document.createElement("p");

            var meta = document.createElement("div");
            meta.className = "row";

            var name = document.createElement("div");
            var strong = document.createElement("strong");
            var nameText = document.createTextNode(file.name);
            strong.appendChild(nameText);
            name.appendChild(strong);
            name.className = "col";
            meta.appendChild(name);

            var filesize = getReadableFileSizeString(file.size);

            var speed = document.createElement("div");
            speed.textContent = "Pending (" + filesize + ")";
            speed.className = "col text-end";
            meta.appendChild(speed);

            // Progressbar
            var bar_row = document.createElement("div");
            bar_row.className = "row";

            var bar_col = document.createElement("div");
            bar_col.className = "col-12";

            var progress = document.createElement("div");
            progress.className = "progress";

            var bar = document.createElement("div");
            bar.className = "progress-bar";

            bar.setAttribute("style", "width: 0%");
            bar.setAttribute("aria-valuemin", 0);
            bar.setAttribute("aria-valuemax", 100);
            bar.setAttribute("aria-valuenow", 0);

            progress.appendChild(bar);
            bar_col.appendChild(progress);
            bar_row.appendChild(bar_col);

            container.appendChild(meta);
            container.appendChild(bar_row);

            fileList.insertBefore(container, fileList.childNodes[0]);
            updateFileCount();
            fileQueue.push({
                file : file,
                container : container
            });
        }
    }

    function humanizeBytesPerSecond(speed) {
        var unit = "KB/s";
        if (speed >= 1024) {
            unit = "MB/s";
            speed /= 1024;
        }
        return (speed.toFixed(1) + unit);
    };

    var uploadFile = function (file, container, retryAttempt) {
        retryAttempt = retryAttempt || 0;
        var maxRetries = 3;

        if (container && file) {
            if (retryAttempt === 0) {
                counter_uploading += 1;
            }

            var filesize = getReadableFileSizeString(file.size);
            var speed = container.getElementsByTagName("div")[2];
            var bar = container.getElementsByTagName("div")[6];

            var xhr = new XMLHttpRequest();
            var upload = xhr.upload;

            // For speed measurements
            var startTime = (new Date()).getTime();

            // Stall detection: retry if no progress for 30 seconds
            var lastProgressTime = startTime;
            var stallCheckInterval = setInterval(function() {
                if ((new Date()).getTime() - lastProgressTime > 30000) {
                    clearInterval(stallCheckInterval);
                    if (!retryUpload("stalled")) {
                        // Final failure after all retries exhausted
                        bar.className = "progress-bar bg-danger";
                        speed.textContent = "Upload stalled (" + filesize + ")";
                        counter_failed += 1;
                        counter_uploading -= 1;
                        updateFileCount();
                    }
                }
            }, 5000);

            // Helper function to retry the upload
            var retryUpload = function(reason) {
                clearInterval(stallCheckInterval);
                if (retryAttempt < maxRetries) {
                    console.log("Upload failed (" + reason + "), retrying... (attempt " + (retryAttempt + 1) + " of " + maxRetries + ")");
                    xhr.abort();
                    bar.className = "progress-bar progress-bar-striped bg-warning";
                    bar.setAttribute("style", "width: 0%");
                    bar.setAttribute("aria-valuenow", 0);
                    speed.textContent = "Retrying... (" + filesize + ")";
                    // Exponential backoff: 1s, 2s, 4s... capped at 30s, plus random jitter
                    var backoffDelay = Math.min(1000 * Math.pow(2, retryAttempt), 30000);
                    var jitter = Math.random() * 500;
                    setTimeout(function() {
                        uploadFile(file, container, retryAttempt + 1);
                    }, backoffDelay + jitter);
                    return true;
                }
                return false;
            };

            // Upload in progress
            upload.addEventListener("progress", function (e) {
                lastProgressTime = (new Date()).getTime();
                if (e.lengthComputable) {
                    bar.className = "progress-bar progress-bar-striped progress-bar-animated";
                    var progress_in_percent = (e.loaded / e.total) * 100;
                    bar.setAttribute("aria-valuenow", progress_in_percent);
                    bar.setAttribute("style", "width: " + progress_in_percent + "%");

                    var speedText;
                    if (e.loaded === e.total && e.total > 0) {
                        // Upload complete
                        speedText = "Server side processing... (" + filesize + ")";
                    } else if (e.loaded > 0) {
                        // Upload in progress
                        var curTime = (new Date()).getTime();
                        var seconds = curTime - startTime;
                        var bps = seconds ? e.loaded / seconds : 0;
                        if (isNaN(bps)) {
                            speedText = "Uploading... (" + filesize + ")";
                        } else {
                            speedText = "Uploading at " + humanizeBytesPerSecond(bps) + " (" + progress_in_percent.toFixed(2) + "% of " + filesize + ")";
                        }
                    } else {
                        // Upload just initiated
                        speedText = "(" + filesize + ")";
                    }

                    speed.textContent = speedText;
                }
            }, false);

            // Upload complete
            xhr.onload = function() {
                clearInterval(stallCheckInterval);
                bar.setAttribute("aria-valuenow", 100);
                var body = xhr.response;
                if (xhr.status === 201 && xhr.readyState === 4) {
                    counter_uploading -= 1;
                    bar.className = "progress-bar bg-success";
                    speed.textContent = "Complete (" + filesize + ")";
                    counter_completed += 1;
                    updateFileCount();
                } else {
                    // Try to retry on server errors (5xx)
                    if (xhr.status >= 500 && retryUpload("status " + xhr.status)) {
                        return;
                    }
                    counter_uploading -= 1;
                    bar.className = "progress-bar bg-danger";
                    speed.textContent = body + " (" + filesize + ")";
                    console.log("Unexpected response code: " + xhr.status);
                    console.log("Response body: " + xhr.response);
                    counter_failed += 1;
                    updateFileCount();
                }
            };

            // Handle upload errors here
            xhr.onerror = function () {
                clearInterval(stallCheckInterval);
                console.log("onerror: status: " + xhr.status + ", readystate: " + xhr.readyState);
                // Try to retry on network errors
                if (retryUpload("network error")) {
                    return;
                }
                bar.className = "progress progress-danger";
                bar.setAttribute("aria-valuenow", 100);
                speed.textContent = "Upload failed (" + filesize + ")";
                counter_failed += 1;
                counter_uploading -= 1;
                updateFileCount();
            };

            // XXX: Consider validating UTF-8 here
            var filename = file.name;

            // XXX: Do this properly using a path join function
            var uploadURL = "/" + bin + "/" + filename;

            console.log("Uploading filename " + filename + " (" + file.size + " bytes) to bin " + bin + " at " + uploadURL + (retryAttempt > 0 ? " (retry " + retryAttempt + ")" : ""));
            xhr.open(
                "POST",
                uploadURL
            );
            xhr.setRequestHeader("Cache-Control", "no-cache");
            xhr.setRequestHeader("X-Requested-With", "XMLHttpRequest");
            xhr.setRequestHeader("Size", file.size);
            xhr.setRequestHeader("Bin", bin);
            xhr.send(file);
        }
    }
}

// http://stackoverflow.com/q/10420352
function getReadableFileSizeString(fileSizeInBytes) {
    var i = -1;
    var byteUnits = ['kB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB'];
    do {
        fileSizeInBytes = fileSizeInBytes / 1024;
        i++;
    } while (fileSizeInBytes > 1024);

    return Math.max(fileSizeInBytes, 0.1).toFixed(1) + byteUnits[i];
};

function deleteURL (url, messageBoxID) {
    console.log("Delete url: " + url);
    var xhr = new XMLHttpRequest();
    var box = document.getElementById(messageBoxID);

    box.textContent = "Delete operation in progress ..."
    box.className = "alert alert-dark";

    xhr.onload = function() {
        if (xhr.status === 200 && xhr.readyState === 4) {
            console.log("Deleted successfully");
            box.textContent = "Delete operation completed successfully.";
            box.className = "alert alert-success";
        } else if (xhr.status === 404 && xhr.readyState === 4) {
            box.textContent = "Delete operation already completed.";
            box.className = "alert alert-success";
        } else {
            console.log("Failed to delete");
            box.textContent = "Error " + xhr.status + ". Unable to verify the operation.";
            box.className = "alert alert-danger";
        }
    };

    xhr.onerror = function () {
        console.log("onerror: status: " + xhr.status + ", readystate: " + xhr.readyState);
    };

    xhr.open(
        "DELETE",
        url
    );

    xhr.send();
};

function lockBin (bin, messageBoxID) {
    console.log("Lock bin: " + bin);
    var xhr = new XMLHttpRequest();
    var box = document.getElementById(messageBoxID);

    box.textContent = "Lock operation in progress ..."
    box.className = "alert alert-dark";

    xhr.onload = function() {
        if (xhr.status === 200 && xhr.readyState === 4) {
            console.log("Locked successfully");
            box.textContent = "Lock operation completed successfully.";
            box.className = "alert alert-success";
        } else if (xhr.status === 409 && xhr.readyState === 4) {
            box.textContent = "The bin is already locked.";
            box.className = "alert alert-success";
        } else {
            console.log("Failed to lock");
            box.textContent = "Error " + xhr.status + ". Unable to verify the operation.";
            box.className = "alert alert-danger";
        }
    };

    xhr.onerror = function () {
        console.log("onerror: status: " + xhr.status + ", readystate: " + xhr.readyState);
    };

    xhr.open(
        "PUT",
        "/" + bin
    );

    xhr.send();
};

function approveBin (bin, messageBoxID) {
    console.log("Approve bin: " + bin);
    var xhr = new XMLHttpRequest();
    var box = document.getElementById(messageBoxID);

    box.textContent = "Approve operation in progress ..."
    box.className = "alert alert-dark";

    xhr.onload = function() {
        if (xhr.status === 200 && xhr.readyState === 4) {
            console.log("Approved successfully");
            box.textContent = "Approved successfully.";
            box.className = "alert alert-success";
        } else if (xhr.status === 409 && xhr.readyState === 4) {
            box.textContent = "The bin is already approved.";
            box.className = "alert alert-success";
        } else {
            console.log("Failed to approve");
            box.textContent = "Error " + xhr.status + ". Unable to verify the operation.";
            box.className = "alert alert-danger";
        }
    };

    xhr.onerror = function () {
        console.log("onerror: status: " + xhr.status + ", readystate: " + xhr.readyState);
    };

    xhr.open(
        "PUT",
        "/admin/approve/" + bin
    );

    xhr.send();
};
