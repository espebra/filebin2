{{ define "apispec" }}{
  "swagger": "2.0",
  "host": "localhost:8080",
  "basePath": "/",
  "schemes": [
    "http",
    "https"
  ],
  "paths": {
    "/{bin}/{filename}": {
      "get": {
        "tags": [
          "file"
        ],
        "summary": "Download a file from a bin",
        "description": "This is a regular file download, which includes content-length and checksums of the content in the response headers. The content-type will be set according to the content.",
        "parameters": [
          {
            "name": "bin",
            "in": "path",
            "description": "The bin to download from.",
            "required": true,
            "type": "string",
          },
          {
            "name": "filename",
            "in": "path",
            "description": "The filename of the file to download from the bin specified.",
            "required": true,
            "type": "string",
          },
        ],
        "responses": {
          "200": {
            "description": "Successful download."
          },
          "404": {
            "description": "The file was not found. The bin may be expired, the file is deleted or it did never exist in the first place."
          }
        }
      },
      "delete": {
        "tags": [
          "file"
        ],
        "summary": "Delete a file from a bin",
        "description": "This will delete a file from a bin. Everyone knowing the URL to the bin have access to deleting files from it.",
        "parameters": [
          {
            "name": "bin",
            "in": "path",
            "description": "The bin to delete from.",
            "required": true,
            "type": "string",
          },
          {
            "name": "filename",
            "in": "path",
            "description": "The filename of the file to delete.",
            "required": true,
            "type": "string",
          },
        ],
        "responses": {
          "200": {
            "description": "The file was successfully flagged for deletion."
          },
          "404": {
            "description": "The file was not found. The bin may be expired or it did never exist in the first place."
          }
        }
      }
    },
    "/": {
      "post": {
        "tags": [
          "file"
        ],
        "summary": "Upload a file to a bin",
        "description": "",
        "parameters": [
          {
            "in": "header",
            "name": "bin",
            "description": "The bin to upload the file to",
            "required": true,
          },
          {
            "in": "header",
            "name": "filename",
            "description": "The filename of the file to upload",
            "required": true,
          },
          {
            "in": "body",
            "name": "body",
            "description": "Content of the file to be uploaded",
            "required": true,
          },
        ],
        "responses": {
          "201": {
            "description": "Successful upload"
          },
          "405": {
            "description": "The bin is locked and can not be written to"
          }
        }
      }
    },
    "/{bin}": {
      "get": {
        "tags": [
          "bin"
        ],
        "summary": "Show a bin",
        "description": "This will show meta data about the bin such as timestamps, file sizes, file names and so on.",
        "parameters": [
          {
            "name": "bin",
            "in": "path",
            "description": "The bin to show.",
            "required": true,
            "type": "string",
          }
        ],
        "produces": [
          "application/json",
        ],
        "responses": {
          "200": {
            "description": "Successful operation"
          },
          "404": {
            "description": "The bin does not exist or is not available"
          }
        }
      },
      "lock": {
        "tags": [
          "bin"
        ],
        "summary": "Lock an entire bin to make it read only",
        "description": "",
        "parameters": [
          {
            "name": "bin",
            "in": "path",
            "description": "The bin to lock.",
            "required": true,
            "type": "string",
          }
        ],
        "responses": {
          "200": {
            "description": "Successful operation"
          },
          "404": {
            "description": "The bin does not exist or is not available"
          }
        }
      },
      "delete": {
        "tags": [
          "bin"
        ],
        "summary": "Delete an entire bin and all of its files",
        "description": "",
        "parameters": [
          {
            "name": "bin",
            "in": "path",
            "description": "The bin to delete.",
            "required": true,
            "type": "string",
          }
        ],
        "responses": {
          "200": {
            "description": "Successful operation"
          },
          "404": {
            "description": "The bin does not exist or is not available"
          }
        }
      },
    }
  }
}
{{ end }}