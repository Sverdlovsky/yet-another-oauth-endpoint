# Yet Another OAuth Endpoint
============================

Just another one OAuth endpoint. Uses Yandex, Google or GitHub as an OAuth 2.0 service. Generates a JWT token with credentials and stores it in cookies for further authentication via your API server. Correct token transfer is required, example:

```js
fetch(url, {
  method: "GET",
  credentials: "include"
})
```

