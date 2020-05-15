{{ define "message" }}<!doctype html>
<html lang="en">
    <head>
        <meta charset="utf-8">
        <meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">
        <link rel="icon" href="/static/img/favicon.png">
        <link rel="stylesheet" href="/static/css/bootstrap.min.css"/>
        <link rel="stylesheet" href="/static/css/fontawesome.all.min.css"/>
        <link rel="stylesheet" href="/static/css/custom.css"/>
        <title>Filebin</title>
    </head>
    <body class="container-xl">

        {{ template "topbar" . }}

        <h1 class="display-1">Filebin</h1>

        <!-- Only show the howto if there are no files in the bin -->
        <p class="lead">
            {{ .Text }}
        </p>

        <p>
            <a href="javascript:history.back()">Go Back</a>
        </p>
 
        <script src="/static/js/jquery-3.4.1.slim.min.js"></script>
        <script src="/static/js/popper.min.js"></script>
        <script src="/static/js/bootstrap.min.js"></script>
    </body>
</html>
{{ end }}
