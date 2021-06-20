function FileAPI (c, t, d, f, bin, binURL, client) {

    var fileCount = c,
        fileList = t,
        dropZone = d,
        fileField = f,
        counter_queue = 0,
        counter_uploading = 0,
        counter_completed = 0,
        counter_failed = 0,
        concurrency = 4,
        fileQueue = new Array(),
        preview = null;

    function showDropZone() {
        dropZone.style.visibility = "visible";
        console.log("Show drop zone");
    }
    function hideDropZone() {
        dropZone.style.visibility = "hidden";
        console.log("Hide drop zone");
    }
    function allowDrag(e) {
        if (true) {  // Test that the item being dragged is a valid one
            e.dataTransfer.dropEffect = 'copy';
            e.preventDefault();
        }
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
        if (counter_queue != 1){
            text = text + "s";
        }
        text = text + " uploaded";
        if (counter_failed > 0) {
            text = text + ". " + counter_failed + " failed, please retry later.";
            box.className = "alert alert-danger";
        } else if (counter_completed == counter_queue) {
            text = text + ", all done!";
            box.className = "alert alert-success";

            // Automatic refresh when uploads complete
            //window.location.assign(binURL)
            window.location.href = binURL;
        }

        if ((counter_completed + counter_failed) != counter_queue) {
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

    this.uploadQueue = function (ev) {
        ev.preventDefault();

        // Loop that will wait 100ms between each iteration
        var i = setInterval(function(){
            // Initiate a upload if within the concurrency limit
            if (counter_uploading < concurrency) {
                var item = fileQueue.pop();
                uploadFile(item.file, item.container);
            }

            // Break out of the loop when the queue is empty
            if (fileQueue.length == 0) {
                clearInterval(i);
            }
        }, 100);
    }

    var addFileListItems = function (files) {
        counter_queue += files.length;
        updateFileCount();
        for (var i = 0; i < files.length; i++) {
            showFileInList(files[i])
        }
    }

    var showFileInList = function (file) {
        //var file = ev.target.file;
        if (file) {
            var container = document.createElement("p");
            //container.className = "list-group-item";

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
            //var size = document.createElement("div");
            //var sizeText = document.createTextNode(filesize);
            //size.appendChild(sizeText);
	    //size.className = "col-md-2";
            //meta.appendChild(size)

            var speed = document.createElement("div");
            //var mimeText = document.createTextNode(mimetype);
            speed.textContent = "Pending (" + filesize + ")";
            speed.className = "col text-end";
            meta.appendChild(speed)

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

            container.appendChild(meta)
            container.appendChild(bar_row)

            fileList.insertBefore(container, fileList.childNodes[0]);
            updateFileCount();
            fileQueue.push({
                file : file,
                container : container
            });
        }
    }

    function roundNumber(num, dec) {
        var result = Math.round(num*Math.pow(10,dec))/Math.pow(10,dec);
        return result;
    }

    function humanizeBytesPerSecond(speed) {
        var unit = "KB/s";
        if (speed >= 1024) {
            unit = "MB/s";
            speed /=1024;
        }
        return (speed.toFixed(1) + unit);
    };

    var uploadFile = function (file, container) {
        if (container && file) {
            counter_uploading += 1;

            var filesize = getReadableFileSizeString(file.size);
            var speed = container.getElementsByTagName("div")[2];
            var bar = container.getElementsByTagName("div")[6];

            var xhr = new XMLHttpRequest();
            upload = xhr.upload;

            // For speed measurements
            var startTime = (new Date()).getTime();

            // Upload in progress
            upload.addEventListener("progress", function (e) {
                if (e.lengthComputable) {
                    bar.className = "progress-bar progress-bar-striped progress-bar-animated";
                    progress_in_percent = (e.loaded / e.total) * 100;
                    bar.setAttribute("aria-valuenow", progress_in_percent);
                    bar.setAttribute("style", "width: " + progress_in_percent + "%");

                    if (e.loaded == e.total && e.total > 0) {
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
            xhr.onload = function(e) {
                bar.setAttribute("aria-valuenow", 100);
                counter_uploading -= 1;
		var body = xhr.response;
                if (xhr.status == 201 && xhr.readyState == 4) {
                    bar.className = "progress-bar bg-success";
                    speed.textContent = "Complete (" + filesize + ")";
                    counter_completed += 1;
                } else {
                    bar.className = "progress-bar bg-danger";
                    speed.textContent = body + " (" + filesize + ")";
                    console.log("Unexpected response code: " + xhr.status);
                    console.log("Response body: " + xhr.response);
                    counter_failed += 1;
                }
                updateFileCount();
            };

            // Handle upload errors here
            xhr.onerror = function (e) {
                console.log("onerror: status: " + xhr.status + ", readystate: " + xhr.readyState);
                console.log(e);
                bar.className = "progress progress-danger";
                bar.setAttribute("aria-valuenow", 100);
                speed.textContent = "Upload failed (" + filesize + ")";
                counter_failed += 1;
                counter_uploading -= 1;
                updateFileCount();
            };

            filename = file.name.replace(/[^A-Za-z0-9-_=,.]/g, "_");

            // XXX: Do this properly using a path join function
            uploadURL = "/" + bin + "/" + filename;

            console.log("Uploading filename " + filename + " (" + file.size + " bytes) to bin " + bin + " at " + uploadURL);
            xhr.open(
                "POST",
                uploadURL
            );
            xhr.setRequestHeader("Cache-Control", "no-cache");
            xhr.setRequestHeader("X-Requested-With", "XMLHttpRequest");
            xhr.setRequestHeader("Filename", filename);
            xhr.setRequestHeader("Size", file.size);
            xhr.setRequestHeader("Bin", bin);
            xhr.setRequestHeader("CID", client);
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

    xhr.onload = function(e) {
        if (xhr.status == 200 && xhr.readyState == 4) {
            console.log("Deleted successfully");
            box.textContent = "Delete operation completed successfully.";
            box.className = "alert alert-success";
        } else if (xhr.status  == 404 && xhr.readyState == 4) {
            box.textContent = "Delete operation already completed.";
            box.className = "alert alert-success";
        } else {
            console.log("Failed to delete");
            box.textContent = "Error " + xhr.status + ". Unable to verify the operation.";
            box.className = "alert alert-danger";
        }
    };

    xhr.onerror = function (e) {
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

    xhr.onload = function(e) {
        if (xhr.status == 200 && xhr.readyState == 4) {
            console.log("Locked successfully");
            box.textContent = "Lock operation completed successfully.";
            box.className = "alert alert-success";
        } else if (xhr.status  == 409 && xhr.readyState == 4) {
            box.textContent = "The bin is already locked.";
            box.className = "alert alert-success";
        } else {
            console.log("Failed to lock");
            box.textContent = "Error " + xhr.status + ". Unable to verify the operation.";
            box.className = "alert alert-danger";
        }
    };

    xhr.onerror = function (e) {
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

    xhr.onload = function(e) {
        if (xhr.status == 200 && xhr.readyState == 4) {
            console.log("Approved successfully");
            box.textContent = "Approved successfully.";
            box.className = "alert alert-success";
        } else if (xhr.status  == 409 && xhr.readyState == 4) {
            box.textContent = "The bin is already approved.";
            box.className = "alert alert-success";
        } else {
            console.log("Failed to approve");
            box.textContent = "Error " + xhr.status + ". Unable to verify the operation.";
            box.className = "alert alert-danger";
        }
    };

    xhr.onerror = function (e) {
        console.log("onerror: status: " + xhr.status + ", readystate: " + xhr.readyState);
    };

    xhr.open(
        "PUT",
        "/admin/approve/" + bin
    );

    xhr.send();
};
