* On unknown page, redirect to referer if possible
* Apparently multiple tunnels can bind to a single server port. Looks like
  maybe only the first one is used to actually tunnel to the clients?
* Responses to unauthorized requests are leaking information about the current
  tunnels through the genereated CSS.
* CSS-only delete buttons don't show up as targets for links like Vimium
  * Wrapping labels in buttons and adding a bit of CSS seems to do the trick.
    * Eh buttons aren't actually doing anything apparently (when hit by
      keyboard).
* See if WebSockets tunnel correctly
* Pretty sure we need to be mutex-locking the cancelFunc calls
* Getting new certs isn't working behind Cloudflare. Might be able to fix by
  using the HTTP challenge and allowing HTTP on the Cloudflare side.
* I think it's possible to create tokens for arbitrary user, even if you're not
  that user.
* Invalid database is wiping out tunnels
* OpenSSH server only picks up the first copy of each key. Will probably need
  to manually combine them for custom keys.
* Send public key back to clients, so they can automatically try to find the
  matching private key.
