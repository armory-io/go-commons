# armory-cloud-iam

This is the Go version of [lib-jvm-armory-cloud-iam](https://github.com/armory-io/lib-jvm-armory-cloud-iam/tree/master/iam-core)

## Core Package

The core package provides a principal service instance that retrieve JWKs from the Armory auth server and verifies the JWT on Armory-specific authorization headers. It accepts the following signing algorithms for JWT verification: RS256, RS384, RS512, PS256, PS384, PS512, ES256, ES384, ES512, ES256K, HS256, HS384, HS512, EdDSA

See the `examples directory to learn how to create the instance and verify the jwt. See [Yeti](https://github.com/armory-io/yeti) for a real world example.

The principal service needs to be instantiated before verification. It is recommended that the JWT public keys url is set in the service's app config, since staging and prod auth servers are at different locations.

```go
err := auth.CreatePrincipalServiceInstance(config.Auth.JWT.JWTKeysUrl); err != nil {
    log.Fatal("failed to initialize principal service")
}
```

This instance can be accessed later with:
```go

psi := iam.GetPrincipalServiceInstance();

```

### JWT Verification: Gorilla Mux Middleware

The `ArmoryCloudPrincipalMiddleware` accepts valid JWTs and rejects requests that do not pass JWT verfication on Armory-specific auth headers. It is up to the service that consumes this library to specify which paths and where in the mux handler chain this middleware will execute in.

```go

http.Handle("/", psi.ArmoryCloudPrincipalMiddleware(r))

```

Pass in optional validators for verifying specific scopes or other properties on the principal required by your service. The validators will execute after the principal is fetched and verified.

```go
// myCustomValidator checks if the user can access deployment information.
func myCustomValidator(p *ArmoryCloudPrincipal) bs {
    if p.HasScope() {
        return nil
    }
    if p.OrgName == "myOrg" {
        return nil
    }
    return errors.New("not authorized")
}
```

The validators need to be passed in when creating the principal service instance.

```go
err := iam.CreatePrincipalServiceInstance(config.Auth.JWT.JWTKeysUrl, myCustomValidator); err != nil {
    log.Fatal("failed to initialize principal service")
}
```

Validators is a variadic argument so multiple parameters can be passed in and will be validated in order.


```go
err := auth.CreatePrincipalServiceInstance(config.Auth.JWT.JWTKeysUrl, myCustomValidator1, myCustomValidator2, myCustomValidator3); err != nil {
    log.Fatal("failed to initialize principal service")
}
```

Example middleware setup:

###  JWT Verification: Manual extraction

You can verify the token after extracting from the authorization headers manually if you do not wish to use the middleware and implement your own bs handling:
```go
token, err := a.ExtractAndVerifyPrincipalFromTokenString(tokenStr)
```