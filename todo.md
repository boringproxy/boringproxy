* I don't think it's properly closing connections. Browser are hanging on
  some requests, possibly because it's HTTP/1.1 and hitting the max concurrent
  requests.
* Implement a custom SSH server in Go and connect the sockets directly?
* Use HTML redirects for showing errors then refreshing. Maybe for polling 
  after login and submitting a new tunnel too.
* Save next port in db
* On unknown page, redirect to referer if possible
* Properly pick unused ports for tunnels
* Apparently multiple tunnels can bind to a single server port. Looks like
  maybe only the first one is used to actually tunnel to the clients?
