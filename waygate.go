package boringproxy

import (
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strings"

	"github.com/takingnames/waygate-go"
)

type WaygateHandler struct {
	db   *Database
	api  *Api
	tmpl *template.Template
	mux  *http.ServeMux
}

func (h *WaygateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func NewWaygateHandler(waygateServer *waygate.Server, db *Database, api *Api) *WaygateHandler {

	tmpl, err := template.ParseFS(fs, "templates/*.tmpl")
	if err != nil {
		fmt.Println(err.Error())
		return nil
	}

	mux := &http.ServeMux{}

	h := &WaygateHandler{
		db:   db,
		api:  api,
		tmpl: tmpl,
	}

	mux.HandleFunc("/authorize", func(w http.ResponseWriter, r *http.Request) {
		h.authorize(w, r)
	})

	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		waygateServer.ServeHTTP(w, r)
	})
	mux.HandleFunc("/open", func(w http.ResponseWriter, r *http.Request) {
		waygateServer.ServeHTTP(w, r)
	})

	h.mux = mux

	return h
}

func (h *WaygateHandler) authorize(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(405)
		fmt.Fprintf(w, "Invalid method")
		return
	}

	r.ParseForm()

	authReq, err := waygate.ExtractAuthRequest(r)
	if err != nil {
		w.WriteHeader(400)
		fmt.Fprintf(w, err.Error())
		return
	}

	wildcardDomains := []string{}

	domains, err := h.api.GetDomainNames(r)
	if err != nil {
		w.WriteHeader(500)
		fmt.Fprintf(w, err.Error())
		return
	}

	for _, domainName := range domains {
		if strings.HasPrefix(domainName, "*.") {
			wildcardDomains = append(wildcardDomains, domainName[2:])
		}
	}

	waygates := h.db.GetWaygates()

	returnUrl := "/waygate" + r.URL.String()

	data := struct {
		Domains     []string
		Waygates    map[string]waygate.Waygate
		AuthRequest *waygate.AuthRequest
		ReturnUrl   string
	}{
		Domains:     wildcardDomains,
		Waygates:    waygates,
		AuthRequest: authReq,
		ReturnUrl:   returnUrl,
	}

	err = h.tmpl.ExecuteTemplate(w, "authorize.tmpl", data)
	if err != nil {
		w.WriteHeader(500)
		fmt.Fprintf(w, err.Error())
		return
	}
}

func (h *WebUiHandler) handleWaygates(w http.ResponseWriter, r *http.Request, user User, tokenData TokenData) {

	r.ParseForm()

	switch r.Method {
	case "GET":
		waygates := h.api.GetWaygates(tokenData)

		templateData := struct {
			User     User
			Waygates map[string]waygate.Waygate
		}{
			User:     user,
			Waygates: waygates,
		}

		err := h.tmpl.ExecuteTemplate(w, "waygates.tmpl", templateData)
		if err != nil {
			w.WriteHeader(500)
			io.WriteString(w, err.Error())
			return
		}
	default:
		w.WriteHeader(405)
		h.alertDialog(w, r, "Invalid method for /waygates", "/waygates")
		return
	}
}

func (h *WebUiHandler) handleWaygateAddNormalDomain(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	domain := r.Form.Get("add-domain")
	h.handleWaygateAddDomain(w, r, domain)
}
func (h *WebUiHandler) handleWaygateAddWildcardDomain(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	host := r.Form.Get("host")
	domain := r.Form.Get("add-wildcard-domain")

	if host != "" {
		domain = fmt.Sprintf("%s.%s", host, domain)
	}

	h.handleWaygateAddDomain(w, r, domain)
}
func (h *WebUiHandler) handleWaygateAddDomain(w http.ResponseWriter, r *http.Request, addDomain string) {
	if r.Method != "POST" {
		w.WriteHeader(405)
		io.WriteString(w, "Invalid method")
		return
	}

	r.ParseForm()

	dup := false
	for _, domain := range r.Form["selected-domains"] {
		if addDomain == domain {
			dup = true
			break
		}
	}

	if !dup {
		r.Form["selected-domains"] = append(r.Form["selected-domains"], addDomain)
	}

	h.handleWaygateEdit(w, r)
}
func (h *WebUiHandler) handleWaygateDeleteSelectedDomain(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(405)
		io.WriteString(w, "Invalid method")
		return
	}

	r.ParseForm()

	deleteDomain := r.Form.Get("delete-domain")

	newSelected := []string{}

	for _, sel := range r.Form["selected-domains"] {
		fmt.Println(sel, deleteDomain)
		if sel != deleteDomain {
			newSelected = append(newSelected, sel)
		}
	}

	r.Form["selected-domains"] = newSelected

	h.handleWaygateEdit(w, r)
}
func (h *WebUiHandler) handleWaygateEdit(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(405)
		io.WriteString(w, "Invalid method")
		return
	}

	r.ParseForm()

	selectedDomains := r.Form["selected-domains"]

	allDomains, err := h.api.GetDomainNames(r)
	if err != nil {
		w.WriteHeader(500)
		fmt.Fprintf(w, err.Error())
		return
	}

	domains := []string{}
	wildcardDomains := []string{}

	for _, domainName := range allDomains {
		if strings.HasPrefix(domainName, "*.") {
			wildcardDomains = append(wildcardDomains, domainName[2:])
		} else {
			domains = append(domains, domainName)
		}
	}

	data := struct {
		SelectedDomains []string
		Domains         []string
		WildcardDomains []string
		ReturnUrl       string
	}{
		SelectedDomains: selectedDomains,
		Domains:         domains,
		WildcardDomains: wildcardDomains,
		ReturnUrl:       r.Form.Get("return-url"),
	}

	err = h.tmpl.ExecuteTemplate(w, "edit_waygate.tmpl", data)
	if err != nil {
		w.WriteHeader(500)
		fmt.Fprintf(w, err.Error())
		return
	}
}

func (h *WebUiHandler) handleWaygateCreate(w http.ResponseWriter, r *http.Request, tokenData TokenData) {
	if r.Method != "POST" {
		w.WriteHeader(405)
		io.WriteString(w, "Invalid method")
		return
	}

	r.ParseForm()

	selectedDomains := r.Form["selected-domains"]
	description := r.Form.Get("description")

	domains, err := h.api.GetDomainNames(r)
	if err != nil {
		w.WriteHeader(500)
		fmt.Fprintf(w, err.Error())
		return
	}

	for _, fqdn := range selectedDomains {
		matched := false
		for _, domain := range domains {
			fmt.Println("comp", fqdn, domain)
			if domain == fqdn {
				matched = true
				break
			} else if strings.HasPrefix(domain, "*.") {
				baseDomain := domain[1:]
				if strings.HasSuffix(fqdn, baseDomain) {
					matched = true
					break
				}
			}
		}

		if !matched {
			w.WriteHeader(403)
			fmt.Fprintf(w, "No permissions for domain")
			return
		}
	}

	wg := waygate.Waygate{
		Domains:     selectedDomains,
		Description: description,
	}

	_, err = h.db.AddWaygate(tokenData.Owner, wg)
	if err != nil {
		w.WriteHeader(500)
		fmt.Fprintf(w, err.Error())
		return
	}

	returnUrl := r.Form.Get("return-url")

	http.Redirect(w, r, returnUrl, 303)
}

func (h *WebUiHandler) handleWaygateConnectExisting(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(405)
		io.WriteString(w, "Invalid method")
		return
	}

	r.ParseForm()

	waygateId := r.Form.Get("waygate-id")

	h.completeAuth(w, r, waygateId)
}

func (h *WebUiHandler) completeAuth(w http.ResponseWriter, r *http.Request, waygateId string) {

	// TODO: Make sure this is secure, ie users can't connect to waygates
	// owned by others.

	waygateToken, err := h.db.AddWaygateToken(waygateId)
	if err != nil {
		w.WriteHeader(500)
		fmt.Fprintf(w, err.Error())
		return
	}

	authReq, err := waygate.ExtractAuthRequest(r)
	if err != nil {
		w.WriteHeader(400)
		io.WriteString(w, err.Error())
		return
	}

	if authReq.RedirectUri == "urn:ietf:wg:oauth:2.0:oob" {
		fmt.Fprintf(w, waygateToken)
	} else {
		code, err := genRandomCode(32)
		if err != nil {
			w.WriteHeader(500)
			fmt.Fprintf(w, err.Error())
			return
		}

		err = h.db.SetTokenCode(waygateToken, code)
		if err != nil {
			w.WriteHeader(500)
			fmt.Fprintf(w, err.Error())
			return
		}
		url := fmt.Sprintf("http://%s?code=%s&state=%s", authReq.RedirectUri, code, authReq.State)
		http.Redirect(w, r, url, 303)
	}
}

func (h *WebUiHandler) confirmDeleteWaygate(w http.ResponseWriter, r *http.Request) {

	r.ParseForm()

	waygateId := r.Form.Get("waygate-id")

	data := &ConfirmData{
		Head:       h.headHtml,
		Message:    "Are you sure you want to delete Waygate?",
		ConfirmUrl: fmt.Sprintf("/waygate-delete?waygate-id=%s", waygateId),
		CancelUrl:  "/waygates",
	}

	err := h.tmpl.ExecuteTemplate(w, "confirm.tmpl", data)
	if err != nil {
		w.WriteHeader(500)
		h.alertDialog(w, r, err.Error(), "/waygates")
		return
	}
}
func (h *WebUiHandler) deleteWaygate(w http.ResponseWriter, r *http.Request, tokenData TokenData) {

	r.ParseForm()

	waygateId := r.Form.Get("waygate-id")

	err := h.api.DeleteWaygate(tokenData, waygateId)
	if err != nil {
		w.WriteHeader(500)
		h.alertDialog(w, r, err.Error(), "/waygates")
		return
	}

	http.Redirect(w, r, "/waygates", 303)
}
