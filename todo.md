# Demo video steps

* Prereqs
  * VPS
    * OpenSSH 7.7+
  * Domain name pointed at public IP

* Todo
  * Upload 0.1.0 release
  * get.sh install script


# 31 Oct 2020 Launch List

- [ ] Client restart on panic
- [ ] Don't require username for client
- [ ] Requires OpenSSH 7.7+ for PermitListen option
- [ ] Finish website
- [ ] Publish releases
- [ ] Demo video
- [ ] Post on /r/selfhosted
- [ ] Improve SSH key download UI.
- [ ] Improve token list UI.
- [ ] Invalid database is wiping out tunnels
- [x] Head can be rendered before h.headHtml is ever set, ie if login page is visited before any other page
- [x] Responses to unauthorized requests are leaking information about the current tunnels through the generated CSS.
- [x] I think it's possible to create tokens for arbitrary user, even if you're not that user.
- [x] Anyone can delete tunnels
- [x] Anyone can delete tokens
- [x] QR codes for admin are broken
- [x] General security review.
- [x] Demo instance
- [x] Demo auto email signup
- [x] Delete tokens when user is deleted


# Eventually 

* On unknown page, redirect to referer if possible
* Apparently multiple tunnels can bind to a single server port. Looks like
  maybe only the first one is used to actually tunnel to the clients?
* CSS-only delete buttons don't show up as targets for links like Vimium
  * Wrapping labels in buttons and adding a bit of CSS seems to do the trick.
    * Eh buttons aren't actually doing anything apparently (when hit by
      keyboard).
* See if WebSockets tunnel correctly
* Getting new certs isn't working behind Cloudflare. Might be able to fix by
  using the HTTP challenge and allowing HTTP on the Cloudflare side.
* We might need some sort of a transaction or atomicity system on the db to
  prevent things like 2 people setting the user at the same time and one losing
  their changes.
* Endpoint for getting user ID from token


# Maybe

* OpenSSH server only picks up the first copy of each key. Will probably need
  to manually combine them for custom keys.
* Send public key back to clients, so they can automatically try to find the
  matching private key.
