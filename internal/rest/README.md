# REST Package

## Overview
This Go package provides utilities for building RESTful APIs. It includes functionality for handling HTTP requests and responses, routing, and more.

## Features
- **API Request/Response Handling**: Utilities for parsing request bodies and encoding responses.
- **Routing**: Dynamic route creation and management using Gorilla Mux.
- **Logging**: Custom logging for request and response handling.
- **Configuration**: Configurable request body size limit via environment variables.

## Installation
To install the package, run:
```bash
go get github.com/plsyro/rest
```

## Usage
### Example: Creating a Simple API
```go
package main

import (
	"net/http"

	"github.com/plsyro/rest/base"
	"github.com/plsyro/rest/router"
)

func main() {
	routes := []router.Route{
		router.CreateRoute(base.GET, "hello", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("Hello, World!"))
		}),
	}

	http.Handle("/", router.NewRouter(routes))
	http.ListenAndServe(":8080", nil)
}
```

### Example: Parsing Request Body
```go
package main

import (
	"net/http"

	"github.com/plsyro/rest/utils/request"
)

func handler(w http.ResponseWriter, r *http.Request) {
	spec, err := request.ParseRequestBody(r, "create", true)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// Process the request body
}
```


## License Information
Copyright (C) Plsyro - All Rights Reserved - 2025

All Plsyro components including this repository are private properties and not allowed to be shared or used for any personal or commercial purposes.
This is fully private Module that holds internal functionalities owned and created by the Plsyro Project Stakeholders and is intended for internal use only. 
Unauthorized copying of the files, via any medium, is strictly prohibited without the express permission of the copyright holders. 
If you encounter the content of this repository and do not have permission, please contact the copyright holders and delete it.

## Contact
For any queries or further assistance, please reach out to the repository maintainer.
Houssem Eddine Kraoua, houssem.kraoua@gmail.com