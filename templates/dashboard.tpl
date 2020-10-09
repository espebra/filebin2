{{ define "dashboard" }}<!doctype html>
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
        <title>Filebin | Dashboard</title>
    </head>
    <body class="container-xl">

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
            </tbody>
        </table>

        <h2>Bins available</h2>
        <table class="table sortable">
            <tr>
                <th>Bin</th>
                <th>Created</th>
                <th>Updated</th>
                <th>Bytes</th>
                <th>Files</th>
                <th>Downloads</th>
                <th>Updates</th>
                <th>Locked</th>
            </tr>
            {{ range $index, $value := .Bins.Available }}
                <tr>
                    <td><code><a href="{{ .URL }}">{{ .Id }}</a></code></td>
                    <td>{{ .CreatedAtRelative }}</td>
                    <td>{{ .UpdatedAtRelative }}</td>
                    <td>{{ .BytesReadable }}</td>
                    <td>{{ .Files }}</td>
                    <td>{{ .Downloads }}</td>
                    <td>{{ .Updates }}</td>
                    <td>
                        {{ if eq .Readonly true }}
                            <i class="fas fa-fw fa-lock text-muted"></i>
                        {{ else }}
                            <i class="fas fa-fw fa-lock-open text-success"></i>
                        {{ end }}
                    </td>
                </tr>
            {{ end }}
        </table>

        <script src="/static/js/popper.min.js"></script>
        <script src="/static/js/bootstrap.min.js"></script>
    </body>
</html>
{{ end }}
