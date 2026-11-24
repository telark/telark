package install

import "k8s.io/apimachinery/pkg/runtime"

type refusalCase struct {
	args   []string
	stderr string
}

type signatureCase struct {
	script string
	line   string
	stderr string
	code   int
}

type stepCase struct {
	installed string
	target    string
	unlisted  bool
	line      string
	code      int
}

type checkCase struct {
	objects []runtime.Object
	refused map[string]int32
	args    []string
	line    string
	code    int
}

type limitedWriter struct {
	left int
}
