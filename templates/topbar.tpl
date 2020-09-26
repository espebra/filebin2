{{ define "topbar" }}

<!--
<nav class="navbar navbar-expand-sm navbar-light">
-->
<nav class="navbar navbar-expand-sm navbar-light d-flex justify-content-between align-items-start">
    <div>
        <ul class="navbar-nav">
	    {{ if eq .Page "front" }}
            {{ else }}
                <li class="nav-item">
                    <a class="nav-link" href="/">Create new bin</a>
                </li>
            {{ end }}
        </ul>
    </div>
    <div class="text-right">
        <ul class="navbar-nav">
	    {{ if eq .Page "bin" }}
                <li class="nav-item">
                    <a class="nav-link" href="" data-toggle="modal" data-target="#modalTakedown">Takedown</a>
                </li>
	    {{ end }}
        </ul>
    </div>
</nav>

{{ if eq .Page "front" }}
{{ else }}
<hr class="mt-0"/>
{{ end }}

<!-- Takedown Modal start -->
<div class="modal fade" id="modalTakedown" tabindex="-1" role="dialog" aria-labelledby="modalTakedownTitle" aria-hidden="true">
    <div class="modal-dialog modal-lg" role="document">
        <div class="modal-content">
            <div class="modal-header alert-secondary">
                <h5 class="modal-title" id="modelTakedownTitle">Takedown</h5>
                <button type="button" class="close" data-dismiss="modal" aria-label="Close">
                    <span aria-hidden="true">&times;</span>
                </button>
            </div>
            <div class="modal-body">
                <p>This web service provides functionality for clients to upload and download files.</p>

                <p>There is an opportunity to abuse this to share illegal, copyrighted or malicious content. There is no automatic moderation of such content, but anyone familiar with the location of the files can delete them at their own will.</p>

                <p>If this bin contains files that should not be here, please <a href="" data-dismiss="modal" data-toggle="modal" data-target="#modalDeleteBin">delete the bin</a>. This will immediately make the files unavailable, and is faster than sending a takedown request to the service owner.</p>
            </div>
            <div class="modal-footer">
                <button type="button" class="btn btn-danger" data-dismiss="modal" data-toggle="modal" data-target="#modalDeleteBin">
                    <i class="fas fa-fw fa-trash-alt"></i> Delete bin
                </button>
                <button type="button" class="btn btn-secondary" data-dismiss="modal">Close</button>
            </div>
        </div>
    </div>
</div>
<!-- Takedown Modal stop -->

{{ end }}
