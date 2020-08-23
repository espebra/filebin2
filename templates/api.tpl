{{ define "api" }}<!doctype html>
<html lang="en">
    <head>
        <meta charset="utf-8">
        <meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">
        <link rel="icon" href="/static/img/favicon.png">
        <link rel="stylesheet" href="/static/css/bootstrap.min.css"/>
        <link rel="stylesheet" href="/static/css/fontawesome.all.min.css"/>
        <link rel="stylesheet" type="text/css" href="/static/css/swagger-ui.css" >
        <script src="/static/js/swagger-ui-bundle.js"> </script>
        <script src="/static/js/swagger-ui-standalone-preset.js"> </script>
        <script>
            window.onload = function() {
                // Begin Swagger UI call region
                const ui = SwaggerUIBundle({
                    url: "/api.yaml",
                    dom_id: '#swagger-ui',
                    deepLinking: true,
                    presets: [
                        SwaggerUIBundle.presets.apis,
                        SwaggerUIStandalonePreset
                    ],
                    supportedSubmitMethods:['get', 'post', 'lock', 'delete'],
                    plugins: [
                        SwaggerUIBundle.plugins.DownloadUrl
                    ],
                    layout: "StandaloneLayout"
                })
                // End Swagger UI call region
                window.ui = ui
            }
        </script>
        <title>Filebin | API documentation</title>
        <style>
            html {
                box-sizing: border-box;
                overflow: -moz-scrollbars-vertical;
                overflow-y: scroll;
            }
      
            *,
            *:before,
            *:after {
                box-sizing: inherit;
            }
      
            body {
                margin:0;
                background: #fafafa;
            }

            .topbar {
                display:none;
            }
            .scheme-container {
                display:none;
            }
        </style>
    </head>

    <body class="container-xl">
        {{template "topbar" .}}
        <h1>API documentation</h1>

        <p>This API documentation page is generated from the <a href="/api.json">OpenAPI 3.0 specification</a> and aims to make it easy to create tools to upload, list, download and delete files from Filebin.</p>

        <div id="swagger-ui"></div>

        {{ template "footer" . }}

        <script src="/static/js/popper.min.js"></script>
        <script src="/static/js/bootstrap.min.js"></script>
    </body>
</html>
{{ end }}
