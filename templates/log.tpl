{{ define "log" }}<!doctype html>
<html lang="en">
    <head>
        <meta charset="utf-8">
        <meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">
        <link rel="icon" href="/static/img/favicon.png">
        <link rel="stylesheet" href="/static/css/bootstrap.min.css"/>
        <link rel="stylesheet" href="/static/css/fontawesome.all.min.css"/>
        <link rel="stylesheet" href="/static/css/custom.css"/>
        <title>Filebin | Log</title>
    </head>
    <body class="container-fluid">

        {{template "adminbar" .}}

        <h1>Log</h1>

        <table class="table table-sm">
            <thead>
                <tr>
                    <th>Timestamp</th>
                    <th>Source IP</th>
                    <th>Client ID</th>
                    <th>Bin</th>
                    <th>Request</th>
                    <th>Request bytes</th>
                    <th>Response bytes</th>
                    <th>Code</th>
                    <th>Duration</th>
                    <th>Details</th>
                </tr>
            </thead>
            <tbody>
                {{ range $index, $value := .Transactions }}
                <tr>
                {{ if eq .Method "POST" }}
                        <tr class="table-secondary">
                {{ end }}
                {{ if eq .Method "GET" }}
                        <tr>
                {{ end }}
                {{ if eq .Method "DELETE" }}
                        <tr class="table-danger">
                {{ end }}
                {{ if eq .Method "PUT" }}
                        <tr class="table-primary">
                {{ end }}
                    <td>
                        <div data-bs-toggle="tooltip" data-bs-placement="right" title="{{ .TimestampRelative }}">
                            {{ .Timestamp.Format "2006-01-02 15:04:05 UTC" }}
                        </div>
                    </td>
                    <td><a href="/admin/log/ip/{{ .IP }}">{{ .IP }}</a></td>
                    <td><a href="/admin/log/cid/{{ .ClientId }}">{{ .ClientId }}</a></td>
                    <td><a href="/admin/log/bin/{{ .BinId }}">{{ .BinId }}</a></td>
                    <td>
                        <code>{{ .Method }} <a href="{{ .Path }}">{{ .Path }}</a></code>
                    </td>
                    <td>
                        {{ .ReqBytesReadable }}
                    </td>
                    <td>
                        {{ .RespBytesReadable }}
                    </td>
                    <td>
                        {{ .Status }}
                    </td>
                    <td>
                        {{ durationInSeconds .Duration }} s
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
