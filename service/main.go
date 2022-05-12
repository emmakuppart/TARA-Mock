package main

import (
	"crypto/rsa"
	"flag"
	"io/ioutil"
	"net/http"
	"sync"

	log "github.com/sirupsen/logrus"

	// Identsustõendi koostamiseks ja allkirjastamiseks
	"github.com/golang-jwt/jwt/v4"
)

// Identsustõendi allkirjastamise RSA võtmepaar
var (
	verifyKey *rsa.PublicKey
	signKey   *rsa.PrivateKey
)

type volituskood string

// Andmed identsustõendi moodustamiseks ja väljastamiseks.
// Identsustõend koostatakse vahetult enne väljastamist.
type forTokenType struct {
	clientID string // autentimispäringust saadud client_id väärtus,
	// tagastatakse identsustõendis, väites (claim) aud (audience)
	sub                  string // subject, isikutõendi väli "sub"
	familyName           string // family_name
	givenName            string // given_name
	state                string // autentimispäringus saadetud turvaväärtus
	nonce                string // autentimispäringus saadetud turvaväärtus
	govSsoLoginChallenge string // autentimispäringus saadetud GOVSSO teenusega seotud turvaväärtus
}

// Identsustõendite hoidla
var idToendid = make(map[volituskood]forTokenType)

var mutex = &sync.RWMutex{}

func main() {

	cFilePtr := flag.String("conf", "config.json", "Seadistusfaili asukoht")
	flag.Parse()

	// Loe seadistus sisse
	conf = loadConf(*cFilePtr)

	// Loe identiteedid sisse
	identities = loadIdentities(conf.IdentitiesFile)

	level, err := log.ParseLevel(conf.LogLevel)
	if err != nil {
		//	Sea vaikimisi (logrus) logitasemeks INFO.
		log.SetLevel(log.InfoLevel)
		// https://qna.habr.com/q/712091 eeskujul.
	} else {
		log.SetLevel(level)
	}

	log.Infoln("** TARA-Mock: Seadistus loetud")

	// Marsruudid
	// Go-s "/" käsitleb ka need teed, millele oma käsitlejat ei leidu.
	http.HandleFunc("/", landingPage)
	http.HandleFunc("/health", healthCheck)
	http.HandleFunc("/.well-known/openid-configuration", sendConf)
	http.HandleFunc("/oidc/.well-known/openid-configuration", sendConf)
	http.HandleFunc("/oidc/authorize", authenticateUser)
	http.HandleFunc("/back", sendUserBack)
	http.HandleFunc("/oidc/token", sendIdentityToken)
	http.HandleFunc("/oidc/jwks", sendKey)

	// Loe sisse identsustõendi allkirjastamise võtmepaar.
	readRSAKeys()

	// fileServer serveerib kasutajaliidese muutumatuid faile.
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// Käivita HTTPS server
	log.Infof("** TARA-Mock: Käivitatud pordil %v", conf.HTTPServerPort)
	err = http.ListenAndServeTLS(
		conf.HTTPServerPort,
		conf.TaraMockCert,
		conf.TaraMockKey,
		nil)
	if err != nil {
		log.Fatal(err)
	}
}

// readRSAKeys loeb sisse identsustõendi allkirjastamise võtmepaari
// ja valmistab ette allkirjastamise avaliku võtme otspunkti.
// Kasutab teeki github.com/golang-jwt/jwt.
func readRSAKeys() {
	signBytes, err := ioutil.ReadFile(conf.IDTokenPrivKeyPath)
	if err != nil {
		log.Fatal(err)
	}

	signKey, err = jwt.ParseRSAPrivateKeyFromPEM(signBytes)
	if err != nil {
		log.Fatal(err)
	}

	verifyBytes, err := ioutil.ReadFile(conf.IDTokenPubKeyPath)
	if err != nil {
		log.Fatal(err)
	}

	verifyKey, err = jwt.ParseRSAPublicKeyFromPEM(verifyBytes)
	if err != nil {
		log.Fatal(err)
	}
}
