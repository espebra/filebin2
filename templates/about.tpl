{{ define "about" }}<!doctype html>
<html lang="en">
    <head>
        <meta charset="utf-8">
        <meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">
        <meta name="description" content="Convenient file sharing. Think of it as Pastebin for files. Registration is not required. Large files are supported.">
        <meta name="author" content="Espen Braastad">
        <link rel="icon" href="/static/img/favicon.png">
        <link rel="stylesheet" href="/static/css/bootstrap.min.css"/>
        <link rel="stylesheet" href="/static/css/fontawesome.all.min.css"/>
        <link rel="stylesheet" href="/static/css/custom.css"/>
        <title>Filebin | About</title>
    </head>
    <body class="container-xl">
        {{template "topbar" .}}
        <h1>About</h1>

        <p>Filebin is a file sharing web application that aims to be convenient and easy to use.</p>

        <p>The <a href="https://github.com/espebra/filebin2/">source code</a> is licensed under the BSD 3-clause license and it can be self hosted. It is built using the following open source components and libraries:</p>
            
        <table class="table table-sm">
            <tr>
                <th>Name</th>
                <th>License</th>
                <th>Source</th>
            </tr>
            <tr>
                <td>Bootstrap</td>
                <td>MIT</td>
                <td><a href="https://getbootstrap.com/">https://getbootstrap.com/</a></td>
            </tr>
            <tr>
                <td>FingerprintJS</td>
                <td>MIT</td>
                <td><a href="https://github.com/fingerprintjs/fingerprintjs">https://github.com/fingerprintjs/fingerprintjs</a></td>
            </tr>
            <tr>
                <td>Font Awesome</td>
                <td>CC BY 4.0, SIL OFL 1.1, MIT License</td>
                <td><a href="https://fontawesome.com/">https://fontawesome.com/</a></td>
            </tr>
            <tr>
                <td>Gorilla Handlers</td>
                <td>BSD 2-clause</td>
                <td><a href="https://github.com/gorilla/handlers/">https://github.com/gorilla/handlers/</a></td>
            </tr>
            <tr>
                <td>Gorilla Mux</td>
                <td>BSD 3-clause</td>
                <td><a href="https://github.com/gorilla/mux/">https://github.com/gorilla/mux/</a></td>
            </tr>
            <tr>
                <td>httpsnoop</td>
                <td>MIT</td>
                <td><a href="https://github.com/felixge/httpsnoop">https://github.com/felixge/httpsnoop</a></td>
            </tr>
            <tr>
                <td>Humane Units</td>
                <td>MIT</td>
                <td><a href="https://github.com/dustin/go-humanize/">https://github.com/dustin/go-humanize/</a></td>
            </tr>
            <tr>
                <td>Mimetype</td>
                <td>MIT</td>
                <td><a href="https://github.com/gabriel-vasile/mimetype/">https://github.com/gabriel-vasile/mimetype/</a></td>
            </tr>
            <tr>
                <td>MinIO Go Client SDK</td>
                <td>Apache 2.0</td>
                <td><a href="https://github.com/minio/minio-go/">https://github.com/minio/minio-go/</a></td>
            </tr>
            <tr>
                <td>Popper</td>
                <td>MIT</td>
                <td><a href="https://popper.js.org/">https://popper.js.org/</a></td>
            </tr>
            <tr>
                <td>Pure Go Postgres driver</td>
                <td>MIT</td>
                <td><a href="https://github.com/lib/pq/">https://github.com/lib/pq/</a></td>
            </tr>
            <tr>
                <td>Rice</td>
                <td>BSD 2-clause</td>
                <td><a href="https://github.com/GeertJohan/go.rice/">https://github.com/GeertJohan/go.rice/</a></td>
            </tr>
        </table>

        {{ template "footer" . }}
    </body>
{{ end }}
