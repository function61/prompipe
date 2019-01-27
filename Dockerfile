FROM scratch

CMD ["/prompipe", "receiver"]

ADD rel/prompipe_linux-amd64 /prompipe
