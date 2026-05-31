// Captured at script-load time. document.currentScript is only readable
// during initial parsing of an external script, so we stash the URL
// here for telemetry to include later. If the script came from a
// different origin than the page (e.g. injected by a proxy that
// rewrote the HTML), the script_host telemetry field will show it.
var filebinScriptURL = (function () {
    try {
        if (document.currentScript && document.currentScript.src) {
            return new URL(document.currentScript.src);
        }
    } catch (e) {}
    return null;
})();

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

            // For telemetry: how far the upload had progressed before failing
            var bytesUploaded = 0;
            // First progress event timestamp; together with startTime this
            // estimates time spent in DNS / TLS / waiting for the server to
            // accept the request — useful for diagnosing slow handshakes.
            var firstProgressTime = 0;
            // Last observed throughput in bytes per second, updated on each
            // progress event. Reported on failure to distinguish "slow link"
            // (low value, recent) from "frozen link" (any value, old).
            var lastBytesPerSecond = 0;

            // Stall detection. While the request body is still being sent we
            // expect frequent progress events, so a short threshold is fine.
            // Once the body is fully sent, progress events stop and we are
            // waiting for the server to checksum, store, and respond — for
            // large files this can legitimately take many minutes, so a much
            // longer threshold applies to avoid retrying an upload the server
            // is still finishing.
            var stallThresholdUploadingMs = 60000;
            var stallThresholdProcessingMs = 900000;
            var uploadBodyComplete = false;
            // Timestamp when the request body finished being sent. Used to
            // compute the server-side processing duration on successful
            // uploads (gap between upload.load and xhr.onload).
            var uploadBodyCompleteTime = 0;
            var lastProgressTime = startTime;
            var stallCheckInterval = setInterval(function() {
                var threshold;
                if (uploadBodyComplete) {
                    threshold = stallThresholdProcessingMs;
                } else {
                    threshold = stallThresholdUploadingMs;
                }
                if ((new Date()).getTime() - lastProgressTime > threshold) {
                    clearInterval(stallCheckInterval);
                    if (!retryUpload("stalled", xhr.status)) {
                        // Final failure after all retries exhausted
                        bar.className = "progress-bar bg-danger";
                        speed.textContent = "Upload stalled (" + filesize + ")";
                        counter_failed += 1;
                        counter_uploading -= 1;
                        reportFailure("stalled", xhr.status);
                        updateFileCount();
                    }
                }
            }, 5000);

            // Posts a telemetry payload to the given endpoint. Uses fetch
            // with keepalive so the request survives page unload (similar
            // to navigator.sendBeacon but with a visible response status),
            // falling back to XHR on browsers without fetch.
            var postTelemetry = function (url, payload) {
                try {
                    var body = JSON.stringify(payload);
                    console.log("Telemetry " + url + ": " + body);
                    if (window.fetch) {
                        fetch(url, {
                            method: "POST",
                            headers: {"Content-Type": "application/json"},
                            body: body,
                            keepalive: true
                        }).then(function (resp) {
                            console.log("Telemetry response: " + resp.status + " " + resp.statusText);
                        }).catch(function (e) {
                            console.log("Telemetry request failed: " + e);
                        });
                    } else {
                        var t = new XMLHttpRequest();
                        t.open("POST", url, true);
                        t.setRequestHeader("Content-Type", "application/json");
                        t.onload = function () {
                            console.log("Telemetry response: " + t.status);
                        };
                        t.onerror = function () {
                            console.log("Telemetry request failed");
                        };
                        t.send(body);
                    }
                } catch (e) {
                    console.log("Failed to send telemetry: " + e);
                }
            };

            // reportFailure submits a terminal-failure telemetry record.
            // Retries are not reported here; retryAttempt is carried along
            // so the server sees how many attempts were tried.
            var reportFailure = function (reason, httpStatus) {
                var now = (new Date()).getTime();
                var conn = navigator.connection || {};
                var respBody = "";
                try {
                    if (xhr.responseType === "" || xhr.responseType === "text") {
                        respBody = xhr.responseText || "";
                    }
                } catch (e) {}
                if (respBody.length > 512) {
                    respBody = respBody.substring(0, 512);
                }
                var contentType = "";
                var requestId = "";
                try {
                    contentType = xhr.getResponseHeader("Content-Type") || "";
                    requestId = xhr.getResponseHeader("X-Request-Id") || "";
                } catch (e) {}
                // Stage at which the failure was detected. handshake = no
                // bytes acknowledged yet (DNS/TLS/server-accept issues);
                // uploading = bytes in flight; awaiting_response = body sent,
                // waiting on server (slow checksum / S3 PUT).
                var stage;
                if (uploadBodyComplete) {
                    stage = "awaiting_response";
                } else if (bytesUploaded > 0) {
                    stage = "uploading";
                } else {
                    stage = "handshake";
                }
                postTelemetry("/api/telemetry/failure", {
                    bin: bin,
                    filename: file.name,
                    upload_host: window.location.host,
                    upload_protocol: window.location.protocol,
                    script_host: filebinScriptURL ? filebinScriptURL.host : "",
                    script_protocol: filebinScriptURL ? filebinScriptURL.protocol : "",
                    top_frame: window.top === window.self,
                    reason: reason,
                    http_status: httpStatus || 0,
                    file_size: file.size,
                    bytes_uploaded: bytesUploaded,
                    duration_ms: now - startTime,
                    time_since_last_progress_ms: now - lastProgressTime,
                    time_to_first_progress_ms: firstProgressTime ? firstProgressTime - startTime : 0,
                    last_bytes_per_second: Math.round(lastBytesPerSecond),
                    retry_attempts: retryAttempt,
                    connection_type: conn.effectiveType || "",
                    stage: stage,
                    ready_state: xhr.readyState,
                    status_text: xhr.statusText || "",
                    online: navigator.onLine !== false,
                    visibility: (document && document.visibilityState) || "",
                    concurrent_uploads: counter_uploading,
                    downlink: conn.downlink || 0,
                    rtt: conn.rtt || 0,
                    save_data: !!conn.saveData,
                    response_body: respBody,
                    response_content_type: contentType.substring(0, 128),
                    request_id: requestId.substring(0, 128)
                });
            };

            // reportSuccess submits a successful-upload telemetry record
            // with phase timings and average throughput.
            var reportSuccess = function () {
                var now = (new Date()).getTime();
                var conn = navigator.connection || {};
                // If upload.load fired we have a clean boundary between
                // uploading and processing phases. If it did not (rare —
                // e.g. very small files on some browsers), fall back to
                // treating the whole duration as uploading.
                var uploadingMs;
                var processingMs;
                if (uploadBodyCompleteTime > 0) {
                    uploadingMs = uploadBodyCompleteTime - startTime;
                    processingMs = now - uploadBodyCompleteTime;
                } else {
                    uploadingMs = now - startTime;
                    processingMs = 0;
                }
                var avgBps = 0;
                if (uploadingMs > 0) {
                    avgBps = Math.round((file.size / uploadingMs) * 1000);
                }
                postTelemetry("/api/telemetry/success", {
                    bin: bin,
                    filename: file.name,
                    upload_host: window.location.host,
                    upload_protocol: window.location.protocol,
                    script_host: filebinScriptURL ? filebinScriptURL.host : "",
                    script_protocol: filebinScriptURL ? filebinScriptURL.protocol : "",
                    top_frame: window.top === window.self,
                    file_size: file.size,
                    duration_ms: now - startTime,
                    uploading_ms: uploadingMs,
                    processing_ms: processingMs,
                    time_to_first_progress_ms: firstProgressTime ? firstProgressTime - startTime : 0,
                    average_bytes_per_second: avgBps,
                    retry_attempts: retryAttempt,
                    connection_type: conn.effectiveType || "",
                    downlink: conn.downlink || 0,
                    rtt: conn.rtt || 0,
                    save_data: !!conn.saveData,
                    visibility: (document && document.visibilityState) || ""
                });
            };

            // Helper that retries the upload. Retries are silent — no
            // telemetry event is sent per attempt. The retry_attempts
            // count is carried into the terminal success or failure event
            // so the server still sees how many attempts were needed.
            var retryUpload = function(category, httpStatus) {
                clearInterval(stallCheckInterval);
                if (retryAttempt < maxRetries) {
                    console.log("Upload failed (" + category + "), retrying... (attempt " + (retryAttempt + 1) + " of " + maxRetries + ")");
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

            // Fires when the request body has been completely sent. After
            // this point, progress events stop, so reset lastProgressTime to
            // measure the (longer) processing threshold from here rather
            // than from the last in-flight progress event.
            upload.addEventListener("load", function () {
                uploadBodyComplete = true;
                uploadBodyCompleteTime = (new Date()).getTime();
                lastProgressTime = uploadBodyCompleteTime;
            }, false);

            // Upload in progress
            upload.addEventListener("progress", function (e) {
                var nowTs = (new Date()).getTime();
                lastProgressTime = nowTs;
                if (firstProgressTime === 0) {
                    firstProgressTime = nowTs;
                }
                if (e.loaded) {
                    bytesUploaded = e.loaded;
                }
                var elapsed = nowTs - startTime;
                if (elapsed > 0 && e.loaded > 0) {
                    lastBytesPerSecond = (e.loaded / elapsed) * 1000;
                }
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
                        var bps = elapsed ? e.loaded / elapsed : 0;
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

            // Centralized cleanup. loadend fires once after any terminal
            // outcome (success, error, abort), so the stall poller is
            // guaranteed to stop here. Individual handlers no longer need to
            // clear the interval themselves.
            xhr.addEventListener("loadend", function () {
                clearInterval(stallCheckInterval);
            }, false);

            // Upload complete
            xhr.onload = function() {
                bar.setAttribute("aria-valuenow", 100);
                var body = xhr.response;
                if (xhr.status === 201 && xhr.readyState === 4) {
                    counter_uploading -= 1;
                    bar.className = "progress-bar bg-success";
                    speed.textContent = "Complete (" + filesize + ")";
                    counter_completed += 1;
                    reportSuccess();
                    updateFileCount();
                } else {
                    // status === 0 in onload typically means a network-level
                    // failure that landed here instead of onerror (CORS
                    // rejection, some proxy disconnects). Treat it as a
                    // network failure rather than an HTTP error so it
                    // retries and is bucketed correctly in telemetry.
                    if (xhr.status === 0) {
                        if (retryUpload("network", 0)) {
                            return;
                        }
                        counter_uploading -= 1;
                        bar.className = "progress progress-danger";
                        bar.setAttribute("aria-valuenow", 100);
                        speed.textContent = "Upload failed (" + filesize + ")";
                        counter_failed += 1;
                        reportFailure("network", 0);
                        updateFileCount();
                        return;
                    }
                    // Try to retry on server errors (5xx)
                    if (xhr.status >= 500 && retryUpload("http_status", xhr.status)) {
                        return;
                    }
                    counter_uploading -= 1;
                    bar.className = "progress-bar bg-danger";
                    speed.textContent = body + " (" + filesize + ")";
                    console.log("Unexpected response code: " + xhr.status);
                    console.log("Response body: " + xhr.response);
                    counter_failed += 1;
                    reportFailure("http_status", xhr.status);
                    updateFileCount();
                }
            };

            // Handle upload errors. A single network drop can fire both
            // xhr.error (response phase) and upload.error (request body
            // phase) — wiring the same handler to both closes a gap seen in
            // Safari while networkErrorHandled prevents double counting and
            // double retries when both fire.
            var networkErrorHandled = false;
            var handleNetworkError = function () {
                if (networkErrorHandled) {
                    return;
                }
                networkErrorHandled = true;
                console.log("network error: status: " + xhr.status + ", readystate: " + xhr.readyState);
                if (retryUpload("network", xhr.status)) {
                    return;
                }
                bar.className = "progress progress-danger";
                bar.setAttribute("aria-valuenow", 100);
                speed.textContent = "Upload failed (" + filesize + ")";
                counter_failed += 1;
                counter_uploading -= 1;
                reportFailure("network", xhr.status);
                updateFileCount();
            };
            xhr.onerror = handleNetworkError;
            upload.addEventListener("error", handleNetworkError, false);

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

function banBin (bin, messageBoxID) {
    console.log("Ban bin: " + bin);
    var xhr = new XMLHttpRequest();
    var box = document.getElementById(messageBoxID);

    box.textContent = "Ban operation in progress ..."
    box.className = "alert alert-dark";

    xhr.onload = function() {
        if (xhr.status === 200 && xhr.readyState === 4) {
            console.log("Banned successfully");
            box.textContent = "Ban operation completed successfully. The bin has been deleted and the uploader IPs have been banned.";
            box.className = "alert alert-success";
        } else {
            console.log("Failed to ban");
            box.textContent = "Error " + xhr.status + ". Unable to verify the operation.";
            box.className = "alert alert-danger";
        }
    };

    xhr.onerror = function () {
        console.log("onerror: status: " + xhr.status + ", readystate: " + xhr.readyState);
    };

    xhr.open(
        "BAN",
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
