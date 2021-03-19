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

        <h2>Bins available</h2>
        <table class="table sortable">
            <tr>
                <th>Bin</th>
                <th>Approved?</th>
                <th>Updated</th>
                <th>Created</th>
                <th>Bytes</th>
                <th>Files</th>
                <th>Downloads</th>
                <th>Updates</th>
                <th>Locked</th>
                <th>Log</th>
            </tr>
            {{ range $index, $value := .Bins.Available }}
                <tr>
                    <td><code><a href="{{ .URL }}">{{ .Id }}</a></code></td>
                    <td>
                        {{ if isApproved . }}
                            <i class="fas fa-fw fa-check-circle text-success"></i>
                        {{ else }}
                            <i class="fas fa-fw fa-circle text-warning"></i> <a href="" data-bs-toggle="modal" data-bs-target="#modalApproveBin-{{ $index }}">Pending</a>
                        {{ end }}
                    </td>
                    <td sorttable_customkey="{{ .UpdatedAt }}">{{ .UpdatedAtRelative }}</td>
                    <td sorttable_customkey="{{ .CreatedAt }}">{{ .CreatedAtRelative }}</td>
                    <td sorttable_customkey="{{ .Bytes }}">{{ .BytesReadable }}</td>
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
                    <td><a href="/admin/log/{{ .Id }}">Log</a></td>
                </tr>
            {{ end }}
        </table>

        <!-- Approve bin modal start -->
        {{ range $index, $value := .Bins.Available }}
            <div class="modal fade" id="modalApproveBin-{{ $index }}" tabindex="-1" role="dialog" aria-labelledby="modalApproveBinTitle" aria-hidden="true">
                <div class="modal-dialog" role="document">
                    <div class="modal-content">
                        <div class="modal-header alert-secondary">
                            <h5 class="modal-title" id="modelApproveBinTitle">Approve bin</h5>
                            <!--<button type="button" class="btn-close" data-bs-dismiss="modal" aria-label="Close"></button>-->
                        </div>
                        <div class="modal-body">
                            {{ if isApproved . }}
                                <p>This bin is already approved.</p>
                            {{ else }}
                                <p>This bin is pending approval. File downloads and archive downloads are not allowed while the bin is pending approval.<p>
                                <p class="lead">Approve bin {{ .Id }}?</p>
                            {{ end }}

                            <div id="approveStatus"></div>
                        </div>
                        <div class="modal-footer">
                            <div class="pull-left">
                            <button type="button" class="btn btn-success" id="approveButton" onclick="approveBin('{{ .Id }}','approveStatus')"><i class="fas fa-fw fa-thumbs-up"></i> Approve</button>
                            </div>
                            <a class="btn btn-secondary" href="/admin">Go back</a>
                        </div>
                    </div>
                </div>
            </div>
        {{ end }}
        <!-- Approve bin modal end -->

        <script src="/static/js/popper.min.js"></script>
        <script src="/static/js/bootstrap.min.js"></script>
    </body>
</html>
{{ end }}
