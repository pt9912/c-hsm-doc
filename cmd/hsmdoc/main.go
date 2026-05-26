// c-hsm-doc-server bootstrap placeholder.
//
// Diese Datei stellt den Build-Pipeline-Anker, bis cmd/hsmdoc den
// echten gRPC-Server bereitstellt. Siehe spec/lastenheft.md
// (HSM-MVP-001) und spec/spezifikation.md.
package main

import (
	"fmt"
	"os"
)

const version = "0.1.0-bootstrap"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "-v":
			fmt.Println("c-hsm-doc-server", version)
			return
		case "--help", "-h":
			fmt.Println("c-hsm-doc-server — bootstrap placeholder")
			fmt.Println("Implementation pending — see spec/lastenheft.md and spec/spezifikation.md.")
			return
		}
	}
	fmt.Fprintln(os.Stderr, "c-hsm-doc-server: bootstrap placeholder, gRPC server implementation pending.")
}
