package main

import (
	/* rsa
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	// "crypto/ed25519"
	"crypto/x509"
	*/
	/* cipher
	 */
	"bufio"
	"fmt"
	"os"
	"strings"

	// "golang.org/x/crypto/ssh/terminal"
	kingpin "github.com/alecthomas/kingpin/v2"
	"github.com/peekjef72/httpapi_exporter/encrypt"
	"github.com/prometheus/common/version"
)

func main() {

	app := kingpin.New("passwd_crypt", "encrypt password wyth a shared key.")
	var (
		decrypt = app.Flag("decrypt", "Decrypt the provided password with key.").Short('d').Default("false").Bool()
		hexa    = app.Flag("hexa", "Encode password in hexastring.(default base64).").Short('x').Default("false").Bool()
	)
	app.HelpFlag.Short('h')
	app.Version(version.Print("passwd_crypt")).VersionFlag.Short('V')
	kingpin.MustParse(app.Parse(os.Args[1:]))
	// kingpin.Parse()

	fmt.Println("give the key: must be 16 24 or 32 bytes long")
	key := credentials("enter key: ")

	cipher, err := encrypt.NewAESCipher(key)
	if err != nil {
		fmt.Printf("%s\n", err)
		os.Exit(1)
	}

	if !*decrypt {
		passwd := credentials("enter password: ")

		fmt.Println("Encrypting...")
		msg := []byte(passwd)
		ciphertext := cipher.Encrypt(msg, !*hexa)
		fmt.Printf("Encrypted message hex: %s\n", ciphertext)
		fmt.Printf("Encrypted message config: 'encrypted/%s'\n", ciphertext)
	} else {
		passwd := credentials("enter encrypted password: ")

		fmt.Println("Decrypting...")
		plaintext, err := cipher.Decrypt(passwd, !*hexa)
		if err != nil {
			// Don't display this message to the end-user, as it could potentially
			// give an attacker useful information. Just tell them something like "Failed to decrypt."
			fmt.Printf("Error decryping message: %s\n", err.Error())
			os.Exit(1)
		}
		fmt.Printf("Decrypted message: %s\n", string(plaintext))
	}
}

func credentials(prompt string) string {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print(prompt)
	res, _ := reader.ReadString('\n')

	// fmt.Print("Enter Password: ")
	// bytePassword, err := terminal.ReadPassword(0)
	// if err == nil {
	// 	fmt.Println("\nPassword typed: " + string(bytePassword))
	// }
	// password := string(bytePassword)

	return strings.TrimSpace(res)
}
