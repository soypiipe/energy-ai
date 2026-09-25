// hashpw genera el hash bcrypt de una contraseña para AUTH_PASSWORD_HASH.
//
//	echo -n 'mi-clave' | go run ./cmd/hashpw
//
// La contraseña se lee de la entrada estándar para que no quede en el historial del shell.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, "leer contraseña:", err)
		os.Exit(1)
	}
	password := strings.TrimRight(string(raw), "\r\n")
	if len(password) < 8 {
		fmt.Fprintln(os.Stderr, "la contraseña debe tener al menos 8 caracteres")
		os.Exit(1)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		fmt.Fprintln(os.Stderr, "generar hash:", err)
		os.Exit(1)
	}
	fmt.Println(string(hash))
}
