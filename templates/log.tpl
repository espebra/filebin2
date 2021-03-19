{{ define "log" }}<!doctype html>
<html lang="en">
    <head>
        <meta charset="utf-8">
        <meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">
        <link rel="icon" href="/static/img/favicon.png">
        <link rel="stylesheet" href="/static/css/bootstrap.min.css"/>
        <link rel="stylesheet" href="/static/css/fontawesome.all.min.css"/>
        <link rel="stylesheet" href="/static/css/upload.css"/>
        <link rel="stylesheet" href="/static/css/custom.css"/>
        <title>Filebin | Log</title>
    </head>
    <body class="container-fluid">

        {{template "adminbar" .}}

        <h1>Log</h1>

	<p class="lead">Transactions logged for bin <a href="/{{ .Bin.Id }}">{{ .Bin.Id }}</a>.</p>

        <table class="table table-sm">
            <thead>
                <tr>
                    <th></th>
                    <th>Timestamp</th>
                    <th>Relative start time</th>
                    <th>IP</th>
                    <th>Request</th>
                    <th>Response bytes</th>
                    <th>Status</th>
                    <th>Duration</th>
                    <th>Details</th>
                </tr>
            </thead>
            <tbody>
                {{ range $index, $value := .Transactions }}
                <tr>
                    <td>
                        {{ if eq .Method "POST" }}
                                <i class="fas fa-cloud-upload-alt text-primary"></i>
                        {{ end }}
                        {{ if eq .Method "GET" }}
                                <i class="fas fa-cloud-download-alt text-success"></i>
                        {{ end }}
                        {{ if eq .Method "DELETE" }}
                                <i class="fas fa-trash-alt text-danger"></i>
                        {{ end }}
                        {{ if eq .Method "PUT" }}
                                <i class="far life-ring text-warning"></i>
                        {{ end }}
                    </td>
                    <td>
                        {{ .Timestamp.Format "2006-01-02 15:04:05 UTC" }}
                    </td>
                    <td>
                        {{ .TimestampRelative }}
                    </td>
                    <td>{{ .IP }}</td>
                    <td>
                        <code>{{ .Method }} <a href="{{ .Path }}">{{ .Path }}</a></code>
                    </td>
                    <td>
                        {{ .RespBytesReadable }}
                    </td>
                    <td>
                        {{ .Status }}
                    </td>
                    <td>
                        {{ .Duration }}
                    </td>
                    <td>
                        <a href="#" data-bs-toggle="collapse" data-bs-target="#collapse{{ $index }}" aria-expanded="false" aria-controls="collapse{{ $index }}"><i class="far fa-window-maximize"></i></a>
                    </td>
                </tr>
                <tr class="collapse" id="collapse{{ $index }}">
                    <td colspan="9">
                        <div class="card">
                            <div class="card-header">
                                    Request headers as submitted by the client
                            </div>
                            <div class="card-body pb-0">
                                    <code><pre>{{- .Headers -}}</pre></code>
                            </div>
                        </div>
                    </td>
                </tr>
                {{ end }}
            </tbody>
        </table>

        <script src="/static/js/popper.min.js"></script>
        <script src="/static/js/bootstrap.min.js"></script>
    </body>
</html>
{{ end }}
