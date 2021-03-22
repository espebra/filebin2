{{ define "admin_dashboard" }}<!doctype html>
<html lang="en">
    <head>
        <meta charset="utf-8">
        <meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">
        <link rel="icon" href="/static/img/favicon.png">
        <link rel="stylesheet" href="/static/css/bootstrap.min.css"/>
        <link rel="stylesheet" href="/static/css/fontawesome.all.min.css"/>
        <link rel="stylesheet" href="/static/css/upload.css"/>
        <link rel="stylesheet" href="/static/css/custom.css"/>
        <script src="/static/js/sorttable.js"></script>
        <script src="/static/js/upload.js"></script>
        <title>Filebin | Dashboard</title>
    </head>
    <body class="container-fluid">

        {{template "adminbar" .}}

        <h1>Dashboard</h1>

        <table class="table">
            <thead>
                <tr>
                    <th></th>
                    <th>Storage (S3) incomplete</th>
                    <th>Storage (S3) current</th>
                    <th>Database current</th>
                    <th>Database total</th>
                </tr>
            </thead>
            <tbody>
                <tr>
                    <th>Number of bytes</th>
                    <td>{{ .BucketInfo.IncompleteObjectsSizeReadable }}</td>
                    <td>{{ .BucketInfo.ObjectsSizeReadable }}</td>
                    <td>{{ .DBInfo.CurrentBytesReadable }}</td>
                    <td>{{ .DBInfo.TotalBytesReadable }}</td>
                </tr>
                <tr>
                    <th>Number of files</th>
                    <td>{{ .BucketInfo.IncompleteObjectsReadable }}</td>
                    <td>{{ .BucketInfo.ObjectsReadable }}</td>
                    <td>{{ .DBInfo.CurrentFilesReadable }}</td>
                    <td>{{ .DBInfo.TotalFilesReadable }}</td>
                </tr>
                <tr>
                    <th>Number of bins</th>
                    <td></td>
                    <td></td>
                    <td>{{ .DBInfo.CurrentBinsReadable }}</td>
                    <td>{{ .DBInfo.TotalBinsReadable }}</td>
                </tr>
                <tr>
                    <th>Log entries</th>
                    <td></td>
                    <td></td>
                    <td>{{ .DBInfo.CurrentLogEntries }}</td>
                    <td></td>
                </tr>
            </tbody>
        </table>

        <script src="/static/js/popper.min.js"></script>
        <script src="/static/js/bootstrap.min.js"></script>
    </body>
</html>
{{ end }}
