package main

import (
	"./url"
	json2 "encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"
)

var (
	porta *int
	logLigado *bool
	urlBase string
)
type Headers map[string]string

type Redirecionador struct {
	stats chan string
}

func init() {
	porta = flag.Int("p", 8888, "porta")
	logLigado = flag.Bool("l", true, "log ligado/desligado")

	flag.Parse()

	urlBase = fmt.Sprintf("http://localhost:%d", *porta)
}

func responderCom(w http.ResponseWriter, status int, headers Headers) {
	for k, v := range headers {
		w.Header().Set(k, v)
	}

	w.WriteHeader(status)
}

func responderComJSON(w http.ResponseWriter, resposta string) {
	responderCom(w, http.StatusOK, Headers{
		"Content-Type": "application/json",
	})
	fmt.Fprintf(w, resposta)
}

func extrairUrl(r *http.Request) string {
	url := make([]byte, r.ContentLength, r.ContentLength)
	r.Body.Read(url)

	return string(url)
}

func buscarUrlEExecutar(w http.ResponseWriter, r *http.Request, executor func(*url.Url)) {
	caminho := strings.Split(r.URL.Path, "/")
	id := caminho[len(caminho) - 1]

	if urlObject := url.Buscar(id); urlObject != nil {
		executor(urlObject)
	}else{
		http.NotFound(w, r)
	}
}

func registrarEstatisticas(ids <- chan string) {
	for id := range ids {
		url.RegistrarClick(id)
		logar("Click registrado com sucesso para %s.", id)
	}
}

func logar(formato string, valores ...interface{}) {
	if *logLigado {
		log.Printf(fmt.Sprintf("%s\n", formato), valores...)
	}
}

func Encurtador(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		responderCom(w, http.StatusMethodNotAllowed, Headers{
			"Allow": "POST",
		})

		return
	}

	url, nova, err := url.BuscarOuCriarNovaUrl(extrairUrl(r))
	if err != nil {
		responderCom(w, http.StatusBadRequest, nil)

		return
	}

	var status int
	if nova {
		status = http.StatusCreated
	}else{
		status = http.StatusOK
	}

	urlCurta := fmt.Sprintf("%s/r/%s", urlBase, url.Id)
	responderCom(w, status, Headers{
		"Location": urlCurta,
		"Link": fmt.Sprintf("<%s/api/stats/%s>; rel=\"stats\"", urlBase, url.Id),
	})

	logar("URL %s encurtada com sucesso para %s.", url.Destino, urlCurta)
}

func Visualizador(w http.ResponseWriter, r *http.Request) {
	buscarUrlEExecutar(w, r, func(url *url.Url) {
		json, err := json2.Marshal(url.Stats())

		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		responderComJSON(w, string(json))
	})
}

func (red *Redirecionador) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	buscarUrlEExecutar(w, r, func(url *url.Url) {
		http.Redirect(w, r, url.Destino, http.StatusMovedPermanently)
		red.stats <- url.Id
	})
}

func main() {
	stats := make(chan string)
	defer close(stats)
	go registrarEstatisticas(stats)

	url.ConfigurarRepositorio(url.NovoRepositorioMemoria())

	http.HandleFunc("/api/encurtar", Encurtador)
	http.Handle("/r/", &Redirecionador{stats})
	http.HandleFunc("/api/stats/", Visualizador)

	logar("Iniciando servidor na porta %d...", *porta)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *porta), nil))
}
