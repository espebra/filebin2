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
    <body class="container-xl">

        {{template "adminbar" .}}

        <h1>Log</h1>

	<p class="lead">Transactions logged for bin <a href="/{{ .Bin.Id }}">{{ .Bin.Id }}</a>.</p>

        <table class="table table-sm">
            <thead>
                <tr>
                    <th></th>
                    <th>Timestamp</th>
                    <th>Relative start time</th>
                    <th>Duration</th>
                    <th>IP</th>
                    <th>Filename</th>
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
                    </td>
                    <td>
                        {{ .StartedAt.Format "2006-01-02 15:04:05 UTC" }}
                    </td>
                    <td>
                        {{ .StartedAtRelative }}
                    </td>
                    <td>
                        {{ if finished .FinishedAt }}
                            {{ elapsed .StartedAt .FinishedAt.Time }}
                        {{ else }}
                            In progress
                        {{ end }}
                    </td>
                    <td>{{ .IP }}</td>
                    <td><a href="/{{ .BinId }}/{{ .Filename }}">{{ .Filename }}</a></td>
                    <td>
                        <a href="#" data-toggle="collapse" data-target="#collapse{{ $index }}" aria-expanded="false" aria-controls="collapse{{ $index }}"><i class="far fa-window-maximize"></i></a>
                    </td>
                </tr>
                <tr class="collapse" id="collapse{{ $index }}">
                    <td colspan="5">
                        <div class="card">
                            <div class="card-header">
                                    Request headers as submitted by the client
                            </div>
                            <div class="card-body pb-0">
                                    <code><pre>{{- .Trace -}}</pre></code>
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
