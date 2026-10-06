# Quickstart: validar el 429 por tokens inexistentes

## Prerrequisitos

- Go 1.25.8, `tfplugindocs` en `$(go env GOPATH)/bin` (lo instala `make generate`).
- Opcional (validación manual): IPzilon >= 3.4.0 local y un token válido (ver la memoria
  *IPzilon local para testacc*).

## 1. Tests unitarios (obligatorio)

```bash
go build ./... && go vet ./... && go test ./...
go test ./internal/client -run 'TestDo_.*InvalidToken|TestDo_Retry' -v
```

Esperado: los tests nuevos comprueban un solo aviso para el 429 de tokens inexistentes, ninguno
para el resto de 429/503, sin token en el log ([contracts/log-warning.md](./contracts/log-warning.md));
los tests de reintento existentes siguen en verde.

## 2. Documentación (obligatorio)

```bash
make generate
git diff --stat docs/        # solo docs/index.md
grep -c "Too many invalid API tokens" docs/index.md README.md
```

Esperado: `docs/index.md` y `README.md` contienen el texto de
[contracts/provider-docs.md](./contracts/provider-docs.md); una segunda ejecución de
`make generate` no deja diferencias.

## 3. Validación manual contra IPzilon 3.4.0 (opcional)

1. Desde la misma máquina, provocar el bloqueo con un token inexistente:

   ```bash
   for i in $(seq 1 31); do
     curl -s -o /dev/null -w '%{http_code}\n' -H 'Authorization: Bearer ipam_inexistente' \
       "$IPZILON_API_URL/api/sites/"
   done   # los últimos devuelven 429
   ```

2. Inmediatamente después, con el token válido:

   ```bash
   TF_LOG=WARN terraform plan 2>&1 | grep -i "non-existent API tokens"
   ```

Esperado: un aviso por petición afectada, el `plan` espera como mucho ~60 s y termina sin error.
