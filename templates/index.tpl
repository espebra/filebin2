{{ define "index" }}<!doctype html>
<html lang="en">
    <head>
        <meta charset="utf-8">
        <meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">
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
                FileAPI = new FileAPI(
                    fileCount,
                    fileList,
                    fileDrop,
                    fileField,
                    bin,
                    uploadURL,
                    binURL
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
            Convenient file sharing without
            registration. Simply upload files and share
            the URL. The files will expire automatically
            {{ .Bin.ExpiredAtRelative }}.
        </p>
        <p class="lead">
            <strong>1.</strong>
            Click <em>Upload files</em> below, or drag and drop the files
            into this browser window.
        </p>
        <p class="lead">
            <strong>2.</strong>
            Wait until the file uploads complete.
        </p>
        <p class="lead">
            <strong>3.</strong>
            Distribute the <a href="/{{ .Bin.Id }}">URL</a> to share access to the files.
        </p>

        <!-- Menu -->
        <p class="fileUpload btn btn-primary">
            <span><i class="fa fa-cloud-upload"></i> Upload files</span>
            <input type="file" class="upload" id="fileField" multiple>
        </p>

        <!-- Drop zone -->
        <div id="fileDrop">Drop files to upload</div>

        <!-- Upload queue -->
        <div id="fileList"></div>

        <!-- Upload status -->
        <div id="fileCount"></div>

        {{ template "footer" . }}

        <script src="/static/js/popper.min.js"></script>
        <script src="/static/js/bootstrap.min.js"></script>
    </body>
</html>
{{ end }}
