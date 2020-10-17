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
* Maybe add a DNS/Domains page and require users to add domains their before
  they can use them for tunnels. This creates a natural place to explain what
  is wrong when domain stuff breaks.
* Responses to unauthorized requests are leaking information about the current
  tunnels through the genereated CSS.
