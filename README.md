# cachingProxy
This software was created out of frustration about the slowness of terraform. As TF does not do any caching at all,
it is very, very slow as it keeps re-requesting the same data. This proxy caches everything based on the full URL.
This speeds up TF a lot. You should not keep this proxy running for longer than just the duration of a single plan, 
as it does not delete from its cache.

Example usage:

    ./cachingProxy &
    HTTP_PROXY=localhost:8080 OS_INSECURE=true terraform plan
    
The `OS_INSECURE` is needed because this proxy does an MITM trick to intercept the HTTPS traffic, and therefor presents
a self-signed cert.