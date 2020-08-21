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
                    <a class="nav-link" href="" data-toggle="modal" data-target="#modalTakedown">Take down</a>
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
            <div class="modal-header">
                <h5 class="modal-title" id="modelTakedownTitle">Takedown</h5>
                <button type="button" class="close" data-dismiss="modal" aria-label="Close">
                    <span aria-hidden="true">&times;</span>
                </button>
            </div>
            <div class="modal-body">
                <p>Takedown</p>
            </div>
            <div class="modal-footer">
                <button type="button" class="btn btn-secondary" data-dismiss="modal">Close</button>
            </div>
        </div>
    </div>
</div>
<!-- Takedown Modal stop -->

{{ end }}
