{{ define "privacy" }}<!doctype html>
<html lang="en">
    <head>
        <meta charset="utf-8">
        <meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">
        <meta name="description" content="Upload files and make them available for your friends. Think of it as Pastebin for files. Registration is not required. Large files are supported.">
        <meta name="author" content="Espen Braastad">
        <link rel="icon" href="/static/img/favicon.png">
        <link rel="stylesheet" href="/static/css/bootstrap.min.css"/>
        <link rel="stylesheet" href="/static/css/fontawesome.all.min.css"/>
        <link rel="stylesheet" href="/static/css/upload.css"/>
        <link rel="stylesheet" href="/static/css/custom.css"/>
        <title>Filebin | Privacy</title>
    </head>
    <body class="container-xl">
        {{template "topbar" .}}
        <h1>Privacy</h1>

	<div class="row">
            <div class="col-md">
                <p>Always be careful when uploading files on the Internet and assume there is no privacy.</p>

                <p>File names and files that are not encrypted prior to being uploaded are readable by the service owner. It is a good idea to encrypt files on the client side before uploading them.</p>

                <p>Files are automatically deleted after a specified period of time. Files can also be deleted manually before this period is up.</p>

                <p>All transactions, including information such as the source IP address of clients uploading and downloading files, are logged for analytics and abuse handling and may be shared with third parties in these contexts.</p>

                <p>This service is using <a href="https://en.wikipedia.org/wiki/HTTPS">HTTPS</a> to secure <a href="https://en.wikipedia.org/wiki/Data_in_transit">data in transit</a> and server side encryption to secure <a href="https://en.wikipedia.org/wiki/Data_at_rest">data at rest</a> (file content). These mechanisms provide data protection from a limited set of scenarios such as man-in-the-middle attacks and data leaks from the object storage service. The content is, however, not encrypted while <a href="https://en.wikipedia.org/wiki/Data_in_use">data in use</a> and the application can decrypt the files as needed for delivery.</p>
            </div>
        </div>

        {{ template "footer" . }}
    </body>
{{ end }}
