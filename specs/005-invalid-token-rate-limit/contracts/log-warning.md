# Contrato: aviso en el log por bloqueo de tokens inexistentes

Interfaz visible: el log del provider (`TF_LOG=WARN` o más detallado, o `TF_LOG_PROVIDER`).

## Cuándo

- Respuesta `429` cuyo mensaje empieza por `Too many invalid API tokens`, **y**
- el cliente va a reintentarla (no se ha agotado el presupuesto), **y**
- es la primera vez dentro de esa petición.

Nunca para otros 429, para 503 ni para el error final.

## Nivel y mensaje

Nivel `WARN`. Mensaje (texto fijo):

```text
IPzilon is temporarily blocking API-token requests from this client address because too many requests with non-existent API tokens came from it; this is not the rate-limit quota of the configured token. Retrying after Retry-After. Check for processes behind the same address (NAT, shared runner) using a deleted or mistyped token, including this provider's own token.
```

## Campos

| Campo | Ejemplo | Notas |
|---|---|---|
| `method` | `GET` | |
| `path` | `/networks/?limit=1000&offset=0` | ruta relativa, como en el Debug actual |
| `status` | `429` | |
| `wait` | `42s` | espera de este reintento (`Duration.String()`) |

**Prohibido**: el token, la cabecera `Authorization` o cualquier otra cabecera.

## Sin cambios

- El `Debug` `retrying IPzilon request` se sigue emitiendo en cada reintento con los mismos campos.
- Esperas, presupuesto de 10 minutos y mensaje del error final (`Too many invalid API tokens
  (gave up after N retries in D)`).
