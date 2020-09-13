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
        <title>Filebin | Dashboard</title>
    </head>
    <body class="container-xl">

        {{template "adminbar" .}}

        <h1>Dashboard</h1>

	<div class="row mb-3">
            <div class="col-sm-3">
                <div class="card text-white bg-primary">
                    <div class="card-body">
                        <p class="card-title">Files stored in S3</p>
                        <h4 class="card-text">{{ .BucketInfo.Objects }}</h4>
                    </div>
                </div>
            </div>
            <div class="col-sm-3">
                <div class="card text-white bg-primary">
                    <div class="card-body">
                        <p class="card-title">Capacity used in S3</p>
                        <h4 class="card-text">{{ .BucketInfo.ObjectsSizeReadable }}</h4>
                    </div>
                </div>
            </div>
            <div class="col-sm-3">
                {{ if eq .BucketInfo.IncompleteObjects 0 }}
                <div class="card text-white bg-success">
                {{ else }}
                <div class="card text-white bg-warning">
                {{ end }}
                    <div class="card-body">
                        <p class="card-title">Incomplete files in S3</p>
                        <h4 class="card-text">{{ .BucketInfo.IncompleteObjects }}</h4>
                    </div>
                </div>
            </div>
            <div class="col-sm-3">
                {{ if eq .BucketInfo.IncompleteObjects 0 }}
                <div class="card text-white bg-success">
                {{ else }}
                <div class="card text-white bg-warning">
                {{ end }}
                    <div class="card-body">
                        <p class="card-title">Size of incomplete files in S3</p>
                        <h4 class="card-text">{{ .BucketInfo.IncompleteObjectsSizeReadable }}</h4>
                    </div>
                </div>
            </div>
        </div>

        <h2>Bins available</h2>
        <table class="table">
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
                    <td><a href="{{ .URL }}">{{ .Id }}</a></td>
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
