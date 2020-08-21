{{ define "bin" }}<!doctype html>
<html lang="en">
    <head>
        <meta charset="utf-8">
        <meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">
        <link rel="icon" href="/static/img/favicon.png">
        <link rel="stylesheet" href="/static/css/bootstrap.min.css"/>
        <link rel="stylesheet" href="/static/css/fontawesome.all.min.css"/>
        <link rel="stylesheet" href="/static/css/upload.css"/>
        <link rel="stylesheet" href="/static/css/custom.css"/>
        <title>Filebin | {{ .Bin.Id }}</title>
        <script src="/static/js/upload.js"></script>
	{{ if eq .Bin.Readonly false }}
        <script>
            window.onload = function () {
                if (typeof FileReader == "undefined") alert ("Your browser \
                    is not supported. You will need to use a \
                    browser with File API support to upload files.");
                var fileCount = document.getElementById("fileCount");
                var fileList = document.getElementById("fileList");
                var fileDrop = document.getElementById("fileDrop");
                var fileField = document.getElementById("fileField");
                var bin = "{{ .Bin.Id }}";
                var uploadURL = "/";
                var binURL = "/{{ .Bin.Id }}";
                FileAPI = new FileAPI(
                    fileCount,
                    fileList,
                    fileDrop,
                    fileField,
                    bin,
                    uploadURL,
                    binURL
                );
                FileAPI.init();
                // Automatically start upload when using the drop zone
                fileDrop.ondrop = FileAPI.uploadQueue;
                // Automatically start upload when selecting files
                if (fileField) {
                    fileField.addEventListener("change", FileAPI.uploadQueue)
                }
            }
        </script>
	{{ end }}
    </head>
    <body class="container-xl">

        {{template "topbar" .}}

        <h1>Filebin</h1>
	{{ if eq .Bin.Readonly false }}
            <!-- Drop zone -->
            <span id="fileDrop">Drop files to upload</span>

            <!-- Upload queue -->
            <span id="fileList"></span>

            <!-- Upload status -->
            <span id="fileCount"></span>
	{{ end }}

        {{ $numfiles := .Files | len }}
        
        <p class="lead">
            The bin <a class="link-primary" href="/{{ .Bin.Id }}">{{ .Bin.Id }}</a> was created {{ .Bin.CreatedRelative }}
            
            {{- if ne .Bin.CreatedRelative .Bin.UpdatedRelative -}}
            , updated {{ .Bin.UpdatedRelative }} 
            {{ end }}
            
            and it expires {{ .Bin.ExpirationRelative }}.
            It contains {{ .Files | len }}

            {{ if eq $numfiles 0 }}files.{{ end }}
            {{ if eq $numfiles 1 }}file at {{ .Bin.BytesReadable }}.{{ end }}
            {{ if gt $numfiles 1 }}files at {{ .Bin.BytesReadable }} in total.{{ end }}
        </p>

        <p>
            <ul class="nav nav-pills">
                {{ if gt $numfiles 0 }}
                    <li class="nav-item mr-3">
                        <a class="nav-link btn btn-primary" href="" data-toggle="modal" data-target="#modalArchive">
                            <i class="fas fa-fw fa-cloud-download-alt"></i> Download files
                        </a>
                    </li>
                {{ end }}
                <li class="nav-item">
                    <div class="dropdown">
                            <a class="nav-link btn btn-primary dropdown-toggle text-white" id="dropdownBinMenuButton" data-toggle="dropdown" aria-haspopup="true" aria-expanded="false">
                                More
                            </a>
                            <div class="dropdown-menu dropdown-menu-right" aria-labelledby="dropdownBinMenuButton">
                                <a class="dropdown-item" href="" data-toggle="modal" data-target="#modalBinProperties" aria-haspopup="true" aria-expanded="false">
                                    <i class="fas fa-fw fa-info-circle text-primary"></i> Bin properties
                                </a>
                                <div class="dropdown-divider"></div>
				{{ if eq .Bin.Readonly false }}
                                <a class="dropdown-item" href="" data-toggle="modal" data-target="#modalLockBin" aria-haspopup="true" aria-expanded="false">
                                    <i class="fas fa-fw fa-lock text-warning"></i> Lock bin
                                </a>
				{{ end }}
                                <a class="dropdown-item" href="" data-toggle="modal" data-target="#modalDeleteBin">
                                    <i class="far fa-fw fa-trash-alt text-danger"></i> Delete bin
                                </a>
                            </div>
                    </div>
                </li>
            </ul>
        </p>

        {{ if .Files }}
            <div class="row">
                <div class="col-5 col-sm-6"><strong>Filename</strong></div>
                <div class="col"><strong>Size</strong></div>
                <div class="col"><strong>Uploaded</strong></div>
                <div class="col"></div>
            </div>
                    {{ range $index, $value := .Files }}
                        <hr class="mt-1 mb-1"/>
                        <div class="row">
                            <div class="col-5 col-sm-6">
                                {{ if eq .Category "image" }}
                                    <i class="far fa-fw fa-file-image"></i>
                                {{ else }}
                                    {{ if eq .Category "video" }}
                                        <i class="far fa-fw fa-file-video"></i>
                                    {{ else }}
                                        <i class="far fa-fw fa-file"></i>
                                    {{ end }}
                                {{ end }}
                                <a class="link-primary" href="{{ .URL }}">{{ .Filename }}</a>
                            </div>
                            <div class="col">{{ .BytesReadable }}</div>
                            <div class="col">{{ .UpdatedRelative }}</div>
                            <div class="col">
                                <div class="dropdown">
                                    <a class="dropdown-toggle small" id="dropdownFileMenuButton" data-toggle="dropdown" aria-haspopup="true" aria-expanded="false">
                                        More
                                    </a>
                                    <div class="dropdown-menu dropdown-menu-right" aria-labelledby="dropdownFileMenuButton">
                                        <a class="dropdown-item" href="#">
                                        <a class="dropdown-item" href="" data-toggle="modal" data-target="#modalFileProperties-{{ $index }}">
                                            <i class="fas fa-fw fa-info-circle text-primary"></i> File properties
                                        </a>
                                        <div class="dropdown-divider"></div>
                                        <a class="dropdown-item" href="" data-toggle="modal" data-target="#modalDeleteFile-{{ $index }}">
                                            <i class="far fa-fw fa-trash-alt text-danger"></i> Delete file
                                        </a>
                                    </div>
                                </div>
                            </div>
                        </div>
                    {{ end }}
        {{ end }}

        <!-- Download archive modal start -->
        <div class="modal fade" id="modalArchive" tabindex="-1" role="dialog" aria-labelledby="modalArchiveTitle" aria-hidden="true">
            <div class="modal-dialog" role="document">
                <div class="modal-content">
                    <div class="modal-header">
                        <h5 class="modal-title" id="modelArchiveTitle">Download files</h5>
                        <button type="button" class="close" data-dismiss="modal" aria-label="Close">
                            <span aria-hidden="true">&times;</span>
                        </button>
                    </div>
                    <div class="modal-body">
                        <p>
                            The files in this bin can be downloaded as a single file archive. The default filename of the archive is <code>{{ .Bin.Id }}</code> and the full size is {{ .Bin.BytesReadable }}.
                        </p>

                        <p class="lead">Select archive format to download:</p>

                        <ul class="nav nav-pills">
                            <li class="nav-item mr-3">
                                <a class="nav-link btn-primary" href="/archive/{{ $.Bin.Id }}/tar"><i class="fas fa-fw fa-file-archive"></i> Tar</a>
                            </li>
                            <li class="nav-item">
                                <a class="nav-link btn-primary" href="/archive/{{ $.Bin.Id }}/zip"><i class="fas fa-fw fa-file-archive"></i> Zip</a>
                            </li>
                        </ul>
                    </div>
                    <div class="modal-footer">
                        <button type="button" class="btn btn-secondary" data-dismiss="modal">Close</button>
                    </div>
                </div>
            </div>
        </div>
        <!-- Download archive modal end -->

        <!-- Delete bin modal start -->
        <div class="modal fade" id="modalDeleteBin" tabindex="-1" role="dialog" aria-labelledby="modalDeleteBinTitle" aria-hidden="true">
            <div class="modal-dialog" role="document">
                <div class="modal-content">
                    <div class="modal-header alert-danger">
                        <h5 class="modal-title" id="modelDeleteBinTitle">Delete bin</h5>
                        <button type="button" class="close" data-dismiss="modal" aria-label="Close">
                            <span aria-hidden="true">&times;</span>
                        </button>
                    </div>
                    <div class="modal-body">
                        <p>You are free to delete this bin. However you are encouraged to delete your own bins only, or bins that are being used to share obvious malicious or illegal content.</p>
                        <p>This action is not reversible.</p>

                        <p class="lead">Delete all the files in bin <a class="link-primary" href="/{{ $.Bin.Id }}">{{ $.Bin.Id }}</a>?</p>

                        <div id="deleteStatus"></div>
                    </div>
                    <div class="modal-footer">
                        <div class="pull-left">
                        <button type="button" class="btn btn-danger" id="deleteButton" onclick="deleteURL('/{{ $.Bin.Id }}','deleteStatus')"><i class="fas fa-fw fa-trash-alt"></i> Confirm</button>
                        </div>
                        <a class="link-primary" href="/{{ $.Bin.Id }}" class="btn btn-secondary"><i class="fa fa-close"></i> Close</a>
                    </div>
                </div>
            </div>
        </div>
        <!-- Delete bin modal end -->

        <!-- Bin properties modal start -->
        <div class="modal fade" id="modalBinProperties" tabindex="-1" role="dialog" aria-labelledby="modalBinPropertiesTitle" aria-hidden="true">
            <div class="modal-dialog modal-lg" role="document">
                <div class="modal-content">
                    <div class="modal-header alert-primary">
                        <h5 class="modal-title" id="modelBinPropertiesTitle">Bin properties</h5>
                        <button type="button" class="close" data-dismiss="modal" aria-label="Close">
                            <span aria-hidden="true">&times;</span>
                        </button>
                    </div>
                    <div class="modal-body">
                        <ul class="row">
                            <dt class="col-sm-3">Bin</dt>
                            <dd class="col-sm-9">
                                <a class="link-primary" href="/{{ $.Bin.Id }}">
                                    {{ $.Bin.Id }}
                                </a>
                            </dd>

                            <dt class="col-sm-3">Number of files</dt>
                            <dd class="col-sm-9">
                                {{ $.Files | len }}
                            </dd>

                            <dt class="col-sm-3">Total size</dt>
                            <dd class="col-sm-9">
                                {{ $.Bin.BytesReadable }}
                                ({{ $.Bin.Bytes }} bytes)
                            </dd>

                            <dt class="col-sm-3">Status</dt>
                            <dd class="col-sm-9">
                                {{ if $.Bin.Readonly }}
					Locked (Read only)
				{{ else }}
					Unlocked
				{{ end }}
                            </dd>

                            <dt class="col-sm-3">Created</dt>
                            <dd class="col-sm-9">
                                {{ $.Bin.CreatedRelative }}
                                ({{ $.Bin.Created.Format "2006-01-02 15:04:05 UTC" }})
                            </dd>

                            <dt class="col-sm-3">Last updated</dt>
                            <dd class="col-sm-9">
                                {{ $.Bin.UpdatedRelative }}
                                ({{ $.Bin.Updated.Format "2006-01-02 15:04:05 UTC" }})
                            </dd>

                            <dt class="col-sm-3">Expires</dt>
                            <dd class="col-sm-9">
                                {{ if $.Bin.ExpirationRelative }}
                                    {{ $.Bin.ExpirationRelative }}
                                {{ end }}
                                ({{ $.Bin.Expiration.Format "2006-01-02 15:04:05 UTC" }})
                            </dd>
                        </ul>
                    </div>
                    <div class="modal-footer">
                        <button type="button" class="btn btn-secondary" data-dismiss="modal">Close</button>
                    </div>
                </div>
            </div>
        </div>
        <!-- Bin properties modal end -->

        <!-- Lock bin modal start -->
        <div class="modal fade" id="modalLockBin" tabindex="-1" role="dialog" aria-labelledby="modalLockBinTitle" aria-hidden="true">
            <div class="modal-dialog" role="document">
                <div class="modal-content">
                    <div class="modal-header alert-warning">
                        <h5 class="modal-title" id="modelLockBinTitle">Lock bin</h5>
                        <button type="button" class="close" data-dismiss="modal" aria-label="Close">
                            <span aria-hidden="true">&times;</span>
                        </button>
                    </div>
                    <div class="modal-body">
                        <p>The bin is currently unlocked, which means that new files can be added to it and existing files can be updated. If the bin is locked, the bin will become read only and no more file uploads will be allowed. Note that a locked bin can still be deleted.</p>
			<p>This action is not reversible.</p>

                        <p class="lead">Do you want to lock bin <a class="link-primary" href="/{{ $.Bin.Id }}">{{ $.Bin.Id }}</a>?</p>

                        <div id="lockStatus"></div>
                    </div>
                    <div class="modal-footer">
                        <div class="pull-left">
                        <button type="button" class="btn btn-warning" id="lockButton" onclick="lockBin('{{ $.Bin.Id }}','lockStatus')"><i class="fas fa-fw fa-lock"></i> Confirm</button>
                        </div>
                        <a class="link-primary" href="/{{ $.Bin.Id }}" class="btn btn-secondary"><i class="fa fa-close"></i> Close</a>
                    </div>
                </div>
            </div>
        </div>
        <!-- Lock bin modal end -->

        <!-- Delete file modal start -->
        {{ range $index, $value := .Files }}
            <div class="modal fade" id="modalDeleteFile-{{ $index }}" tabindex="-1" role="dialog" aria-labelledby="modalDeleteFileTitle" aria-hidden="true">
                <div class="modal-dialog" role="document">
                    <div class="modal-content">
                        <div class="modal-header alert-danger">
                            <h5 class="modal-title" id="modelDeleteFileTitle">Delete file</h5>
                            <button type="button" class="close" data-dismiss="modal" aria-label="Close">
                                <span aria-hidden="true">&times;</span>
                            </button>
                        </div>
                        <div class="modal-body">
                            <p>You are free to delete any file in this bin. However you are encouraged to delete the files that you have uploaded only, or files with obvious malicious or illegal content.</p>
                            <p>This action is not reversible.</p>

                            <p class="lead">Delete the file <a class="link-primary" href="/{{ $.Bin.Id }}/{{ .Filename }}">{{ .Filename }}</a>?</p>

                            <div id="deleteStatus-{{ $index }}"></div>
                        </div>
                        <div class="modal-footer">
                            <div class="pull-left">
                            <button type="button" class="btn btn-danger" id="deleteButton" onclick="deleteURL('/{{ $.Bin.Id }}/{{ .Filename }}','deleteStatus-{{ $index }}')"><i class="fas fa-fw fa-trash-alt"></i> Confirm</button>
                            </div>
                            <a class="link-primary" href="/{{ $.Bin.Id }}" class="btn btn-secondary"><i class="fa fa-close"></i> Close</a>
                        </div>
                    </div>
                </div>
            </div>
        {{ end }}
        <!-- Delete file modal end -->

        <!-- File properties modal start -->
        {{ range $index, $value := .Files }}
            <div class="modal fade" id="modalFileProperties-{{ $index }}" tabindex="-1" role="dialog" aria-labelledby="modalFilePropertiesTitle" aria-hidden="true">
                <div class="modal-dialog modal-lg" role="document">
                    <div class="modal-content">
                        <div class="modal-header alert-primary">
                            <h5 class="modal-title" id="modelFilePropertiesTitle">File properties</h5>
                            <button type="button" class="close" data-dismiss="modal" aria-label="Close">
                                <span aria-hidden="true">&times;</span>
                            </button>
                        </div>
                        <div class="modal-body">
                            <ul class="row">
                                <dt class="col-sm-3">Filename</dt>
                                <dd class="col-sm-9">
                                    <a class="link-primary" href="{{ .URL }}">
                                        {{ .Filename }}
                                    </a>
                                </dd>

                                <dt class="col-sm-3">Bin</dt>
                                <dd class="col-sm-9">
                                    <a class="link-primary" href="/{{ $.Bin.Id }}">
                                        {{ $.Bin.Id }}
                                    </a>
                                </dd>

                                <dt class="col-sm-3">File size</dt>
                                <dd class="col-sm-9">
                                    {{ .BytesReadable }}
                                    ({{ .Bytes }} bytes)
                                </dd>

                                {{ if ne .Created .Updated }}
                                    <dt class="col-sm-3">Update count</dt>
                                    <dd class="col-sm-9">
                                        {{ .Updates }}
                                    </dd>

                                    <dt class="col-sm-3">Last updated</dt>
                                    <dd class="col-sm-9">
                                        {{ .UpdatedRelative }}
                                        ({{ .Updated.Format "2006-01-02 15:04:05 UTC" }})
                                    </dd>
                                {{ end }}

                                <dt class="col-sm-3">Created</dt>
                                <dd class="col-sm-9">
                                    {{ .CreatedRelative }}
                                    ({{ .Created.Format "2006-01-02 15:04:05 UTC" }})
                                </dd>

                                <dt class="col-sm-3">Expires</dt>
                                <dd class="col-sm-9">
                                    {{ if $.Bin.ExpirationRelative }}
                                        {{ $.Bin.ExpirationRelative }}
                                    {{ end }}
                                    ({{ $.Bin.Expiration.Format "2006-01-02 15:04:05 UTC" }})
                                </dd>
                            </ul>
                        </div>
                        <div class="modal-footer">
                            <a class="link-primary" href="/{{ $.Bin.Id }}" class="btn btn-secondary"><i class="fa fa-close"></i> Close</a>
                        </div>
                    </div>
                </div>
            </div>
        {{ end }}
        <!-- File properties modal end -->

        <script src="/static/js/popper.min.js"></script>
        <script src="/static/js/bootstrap.min.js"></script>
    </body>
</html>
{{ end }}
