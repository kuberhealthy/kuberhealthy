package main

import (
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strconv"

	log "github.com/sirupsen/logrus"
	"k8s.io/api/admission/v1beta1"
)

const (
	httpHeaderContentType                = "Content-Type"
	httpHeaderContentTypeApplicationJSON = "application/json"
)

type webhook struct {
	server *http.Server
}

func startServer(wh *webhook) {
	for {
		log.Infoln("Starting web server. Listening to port " + wh.server.Addr + ".")
		err := wh.server.ListenAndServeTLS("", "")
		if err != nil {
			log.Errorln("Failed to start web server:", err.Error())
		}
	}
}

func (wh *webhook) serve(w http.ResponseWriter, r *http.Request) {
	log.Infoln("Handling dynamic admission request from", r.UserAgent()+".")

	contentType := r.Header.Get(httpHeaderContentType)
	if contentType != httpHeaderContentTypeApplicationJSON {
		log.Errorln("Failed to process request body from", r.UserAgent(), "because request content type was not", httpHeaderContentTypeApplicationJSON)
		http.Error(w, "Failed to process request body from "+r.UserAgent()+" because request content type was not "+httpHeaderContentTypeApplicationJSON, http.StatusBadRequest)
		return
	}

	requestBody, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Errorln("Unable to read request body from", r.UserAgent()+":", err.Error())
		http.Error(w, "Unable to read request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if len(requestBody) == 0 {
		log.Errorln("Received an empty body from requester.")
		http.Error(w, "Received an empty request body.", http.StatusBadRequest)
		return
	}
	log.Infoln("Handling " + strconv.Itoa(len(requestBody)) + " bytes.")

	admResp := &v1beta1.AdmissionResponse{}
	review := &v1beta1.AdmissionReview{}
	_, version, err := deserializer.Decode(requestBody, nil, review)
	if err != nil {
		log.Errorln("Unable to decode request body:", err.Error())
	}
	log.Debugln("Decoded request", review.Request.UID, "on api version:", version)
	switch r.URL.Path {
	case mutatePath:
		log.Infoln("Mutating request", review.Request.UID+".")
		admResp = wh.mutate(review)
	case validatePath:
		log.Infoln("Validating request", review.Request.UID+".")
		admResp = wh.validate(review)
	default:
		log.Infoln("Received a request against an unexpected path. Ignoring request", review.Request.UID+".")
		return
	}

	finalReview := &v1beta1.AdmissionReview{}
	if admResp != nil {
		finalReview.Response = admResp
		if review.Request != nil {
			finalReview.Response.UID = review.Request.UID
		}
	}

	resp, err := json.Marshal(finalReview)
	if err != nil {
		log.Errorln("Unable to encode admission review to response body:", err.Error())
		http.Error(w, "Unable to encode admission review: "+err.Error(), http.StatusInternalServerError)
	}

	_, err = w.Write(resp)
	if err != nil {
		log.Errorln("Unable to write response to body:", err.Error())
		http.Error(w, "Unable to write resposne to body: "+err.Error(), http.StatusInternalServerError)
	}
	log.Infoln("Server validated request from "+r.UserAgent()+". ("+strconv.Itoa(len(requestBody))+" bytes).", "[UID:", finalReview.Response.UID+"]")
}

// loadTLS loads TLS PEM encoded data. Expects files / paths.
func loadTLS(keyPath, certPath string) tls.Certificate {
	log.Debugln("Loading TLS certificate and key pair at [key:" + keyPath + "] and [certificate:" + certPath + "]")
	pair, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		log.Fatalln("unable to load TLS key pair:", err.Error())
	}
	return pair
}
