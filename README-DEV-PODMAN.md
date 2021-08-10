```

```




```
podman run --rm -dti -v $PWD/:/app:Z  --net host --name golang golang:1.16.5-alpine  sh
podman exec -ti golang sh
podman rm -fv golang



```
