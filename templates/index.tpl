{{ define "index" }}<!doctype html>
<html lang="en">
    <head>
        <meta charset="utf-8">
        <meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">
        <meta name="description" content="Convenient file sharing. Think of it as Pastebin for files. Registration is not required. Large files are supported.">
        <meta name="author" content="Espen Braastad">
        <link rel="icon" href="/static/img/favicon.png">
        <link rel="stylesheet" href="/static/css/bootstrap.min.css"/>
        <link rel="stylesheet" href="/static/css/fontawesome.all.min.css"/>
        <link rel="stylesheet" href="/static/css/upload.css"/>
        <link rel="stylesheet" href="/static/css/custom.css"/>
        <title>Filebin</title>
        <script src="/static/js/upload.js"></script>
        <script>
            window.onload = function () {
                if (typeof FileReader == "undefined") alert ("Your browser \
                    is not supported. You will need to use a \
                    browser with File API support to upload files.");
                var fileCount = document.getElementById("fileCount");
                var fileList = document.getElementById("fileList");
                var fileDrop = document.getElementById("fileDrop");
                var fileField = document.getElementById("fileField");
                var bin = "{{ .Bin.Id }}";
                var uploadURL = "/";
                var binURL = "/{{ .Bin.Id }}";
                var client = new ClientJS();
                FileAPI = new FileAPI(
                    fileCount,
                    fileList,
                    fileDrop,
                    fileField,
                    bin,
                    uploadURL,
                    binURL,
                    client.getFingerprint()
                );
                FileAPI.init();
                // Automatically start upload when using the drop zone
                fileDrop.ondrop = FileAPI.uploadQueue;
                // Automatically start upload when selecting files
                if (fileField) {
                    fileField.addEventListener("change", FileAPI.uploadQueue)
                }
            }
        </script>
    </head>
    <body class="container-xl">

        {{ template "topbar" . }}

        <h1 class="display-1">Filebin</h1>

        <!-- Only show the howto if there are no files in the bin -->
        <p class="lead">
            Convenient file sharing in three steps without
            registration.
        </p>
        <p class="lead pt-1">
            <strong class="mt-2 pe-2"><span class="rounded-pill bg-secondary btn btn-sm text-light" style="width: 2rem; height:2rem;">1</span></strong>
            <br class="mobile-break">
            <span class="fileUpload btn btn-primary mt-2 mb-2"><label>Select files to upload<input type="file" class="upload" id="fileField" multiple></label></span> or <em>drag-and-drop</em> files into this browser window.
        </p>
        <p class="lead">
            <strong class="mt-2 pe-2"><span class="rounded-pill bg-secondary btn btn-sm text-light" style="width: 2rem; height:2rem;">2</span></strong>
            <br class="mobile-break">
            <span>Wait until the file uploads complete.</span>
        </p>
        <p class="lead">
            <strong class="mt-2 pe-2"><span class="rounded-pill bg-secondary btn btn-sm text-light" style="width: 2rem; height:2rem;">3</span></strong>
            <br class="mobile-break">
            <span>The files will be available at <a href="{{ .Bin.URL }}">{{ .Bin.URL }}</a> which is a link you can share.</span>
        </p>

        <p class="pt-3 text-muted">
            <em>
            The files can be deleted manually at any time and will in any case be deleted automatically {{ .Bin.ExpiredAtRelative }}.
            </em>
        </p>

        {{ if eq .AvailableStorage false }}
            <div class="alert alert-warning" role="alert">
                The storage is currently full. Please come back later.
            </div>
        {{ end }}

        <!-- Drop zone -->
        <div id="fileDrop">Drop files to upload</div>

        <!-- Upload queue -->
        <div id="fileList"></div>

        <!-- Upload status -->
        <div id="fileCount"></div>

        {{ template "footer" . }}

        <script src="/static/js/popper.min.js"></script>
        <script src="/static/js/bootstrap.min.js"></script>
        <script src="/static/js/client.min.js"></script>
    </body>
</html>
{{ end }}
