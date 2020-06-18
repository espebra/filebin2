{{ define "topbar" }}
<nav class="navbar navbar-expand-sm navbar-light d-flex justify-content-between align-items-start small">
    <div>
        <ul class="navbar-nav">
            <li class="nav-item">
                <a class="nav-link" href="/">Front page</a>
            </li>
        </ul>
    </div>
    <div class="text-right">
        <button class="navbar-toggler" type="button" data-toggle="collapse" data-target="#navbarItems" aria-controls="navbarItems" aria-expanded="false" aria-label="Toggle navigation">
            <span class="navbar-toggler-icon btn-sm"></span>
        </button>
        <div class="collapse navbar-collapse" id="navbarItems">
            <ul class="navbar-nav">
                <li class="nav-item">
                    <a class="nav-link" href="/about">About</a>
                </li>
                <li class="nav-item">
                    <a class="nav-link" href="/api">API</a>
                </li>
                <li class="nav-item">
                    <a class="nav-link" href="" data-toggle="modal" data-target="#modalSecurity">Security</a>
                </li>
                <li class="nav-item">
                    <a class="nav-link" href="" data-toggle="modal" data-target="#modalTakedown">Take down</a>
                </li>
                <li class="nav-item">
                    <a class="nav-link" href="" data-toggle="modal" data-target="#modalTerms">Terms and conditions</a>
                </li>
            </ul>
        </div>
    </div>
</nav>

<hr class="mt-0"/>

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

<!-- About Modal start -->
<div class="modal fade" id="modalAbout" tabindex="-1" role="dialog" aria-labelledby="modalAboutTitle" aria-hidden="true">
    <div class="modal-dialog modal-lg" role="document">
        <div class="modal-content">
            <div class="modal-header">
                <h5 class="modal-title" id="modelAboutTitle">About</h5>
                <button type="button" class="close" data-dismiss="modal" aria-label="Close">
                    <span aria-hidden="true">&times;</span>
                </button>
            </div>
            <div class="modal-body">
                <p>Filebin is a file sharing service that is easy to use.</p>

                <p>It is licensed under the BSD 3-clause license. The <a href="https://github.com/espebra/filebin2/">source code is available on Github</a>. It is built using the following open source components and libraries, which are bundled in the Filebin source code repository:</p>
                    
                <table class="table">
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
                        <td>MinIO Secure IO</td>
                        <td>Apache 2.0</td>
                        <td><a href="https://github.com/minio/sio/">https://github.com/minio/sio/</a></td>
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
            </div>
            <div class="modal-footer">
                <button type="button" class="btn btn-secondary" data-dismiss="modal">Close</button>
            </div>
        </div>
    </div>
</div>
<!-- About Modal stop -->

<!-- Security Modal start -->
<div class="modal fade" id="modalSecurity" tabindex="-1" role="dialog" aria-labelledby="modalSecurityTitle" aria-hidden="true">
    <div class="modal-dialog modal-lg" role="document">
        <div class="modal-content">
            <div class="modal-header">
                <h5 class="modal-title" id="modelSecurityTitle">Security</h5>
                <button type="button" class="close" data-dismiss="modal" aria-label="Close">
                    <span aria-hidden="true">&times;</span>
                </button>
            </div>
            <div class="modal-body">
                <p>Security</p>
            </div>
            <div class="modal-footer">
                <button type="button" class="btn btn-secondary" data-dismiss="modal">Close</button>
            </div>
        </div>
    </div>
</div>
<!-- Security Modal stop -->

<!-- Terms and conditions Modal start -->
<div class="modal fade" id="modalTerms" tabindex="-1" role="dialog" aria-labelledby="modalTermsTitle" aria-hidden="true">
    <div class="modal-dialog modal-lg" role="document">
        <div class="modal-content">
            <div class="modal-header">
                <h5 class="modal-title" id="modelTermsTitle">Terms and conditions</h5>
                <button type="button" class="close" data-dismiss="modal" aria-label="Close">
                    <span aria-hidden="true">&times;</span>
                </button>
            </div>
            <div class="modal-body">
                <p>Terms and conditions</p>
            </div>
            <div class="modal-footer">
                <button type="button" class="btn btn-secondary" data-dismiss="modal">Close</button>
            </div>
        </div>
    </div>
</div>
<!-- Privacy Modal stop -->

{{ end }}
