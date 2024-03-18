# Traefik Cookie Path Replacement Middleware

This is a plugin for [Traefik](https://traefik.io) that allows you to rewrite the path of cookies.
## Configuration

### Static

```yaml
experimental:
  plugins:
    cookiePathReplacement:
      modulename: "github.com/JoshuaBowerman/TraefikCookiePathReplacement"
      version: "v0.0.1"
```

### Dynamic

To configure the  plugin you should create a [middleware](https://docs.traefik.io/middlewares/overview/) in your dynamic configuration as explained [here](https://docs.traefik.io/middlewares/overview/).
The following example uses the new middleware to replace the "/old/" part of the path of a cookie with a name starting with test with "/new/".

```yaml
http:
  middlewares:
    cookiePathReplacement:
      plugin:
        cookiePathReplacement:
          replacements:
            - name: "test(.*)"
              old: "(?P<before>.*)/old/(?P<after>.*)"
              new: "{{before}}/new/{{after}}"
```
