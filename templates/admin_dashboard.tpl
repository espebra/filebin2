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

        <link rel="stylesheet" href="/static/css/Chart.min.css"/>
        <script src="/static/js/Chart.min.js"></script>

        <title>Filebin | Dashboard</title>
    </head>
    <body class="container-fluid">

        {{template "adminbar" .}}

        <h1>Dashboard</h1>

        <div class="row">
            <div class="col-sm-3">
                <div class="card">
                    {{ if gt .Config.LimitStorageBytes 0 }}
                        {{ if eq .DBInfo.FreeBytes 0 }}
                            <div class="card-header bg-warning text-center">Storage utilization</div>
                        {{ else }}
                            <div class="card-header text-center">Storage utilization</div>
                        {{ end }}
                    {{ else }}
                        <div class="card-header text-center">Storage utilization</div>
                    {{ end }}
                    <div class="card-body">
                        <p class="card-text">
                            <canvas id="storage" width="200" height="200"></canvas>
                        </p>
                    </div>
                </div>
            </div>

            <div class="col-sm-3">
                <div class="card">
                  <div class="card-body">
                    <h5 class="card-title text-center">Placeholder</h5>
                    <p class="card-text">
                        Placeholder
                    </p>
                  </div>
                </div>
            </div>

            <div class="col-sm-3">
                <div class="card">
                  <div class="card-body">
                    <h5 class="card-title text-center">Placeholder</h5>
                    <p class="card-text">
                        Placeholder
                    </p>
                  </div>
                </div>
            </div>

            <div class="col-sm-3">
                <div class="card">
                  <div class="card-body">
                    <h5 class="card-title text-center">Placeholder</h5>
                    <p class="card-text">
                        Placeholder
                    </p>
                  </div>
                </div>
            </div>
        </div>

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

        <script>
            /* Storage utilization pie chart */
            var storage = document.getElementById('storage');
            new Chart(storage, {
                type: 'doughnut',
                data: {
                    labels: ['Used ({{ .DBInfo.CurrentBytesReadable }})', 'Free {{ if eq .Config.LimitStorageBytes 0 }}(Unlimited){{ else }}({{ .DBInfo.FreeBytesReadable }} of {{ .Config.LimitStorageReadable }}){{ end }}'],
                    datasets: [{
                        label: 'Storage utilization',
                        data: [
                            {{ .DBInfo.CurrentBytes }},
                            {{ .DBInfo.FreeBytes }}
                        ],
                        backgroundColor: [
                            '#0d6efd',
                            '#198754'
                        ],
                        borderColor: [
                            '#ffffff',
                            '#ffffff'
                        ],
                        borderWidth: 1
                    }]
                },
                options: {}
            });
        </script>
    </body>
</html>
{{ end }}
