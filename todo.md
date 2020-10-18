* On unknown page, redirect to referer if possible
* Apparently multiple tunnels can bind to a single server port. Looks like
  maybe only the first one is used to actually tunnel to the clients?
* Responses to unauthorized requests are leaking information about the current
  tunnels through the genereated CSS.
* CSS-only delete buttons don't show up as targets for links like Vimium
  * Wrapping labels in buttons and adding a bit of CSS seems to do the trick.
    * Eh buttons aren't actually doing anything apparently (when hit by
      keyboard).
* Set Cache-Control Max-Age
