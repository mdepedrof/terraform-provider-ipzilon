# Research: Adaptación del provider a IPzilon 3.0.0

**Feature**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md) | **Fecha**: 2026-09-28

Fuentes: informe *Cambios de contrato de la API — IPzilon 3.0.0* (`§N`) y código de IPzilon
en la rama `feat/002-performance-scale-audit` (verificado donde se indica).

## R1. Recorrido de páginas

- **Decision**: función genérica `GetAll[T any](ctx, c *Client, path string) ([]T, error)` en
  `internal/client`, que añade `limit=1000&offset=N` respetando la query existente (`?` o `&`),
  deserializa `Page[T]{Items []T; Total int}` y acumula hasta `offset >= total` o página vacía.
  Si la API devuelve `items: []` con `total > 0` (inventario que encoge durante el recorrido),
  termina igualmente. Límite de seguridad: si el número de páginas supera `ceil(total/1000)+1`,
  se corta con error para no entrar en bucle.
- **Rationale**: un único punto de paginación para los 6 data sources (Principio de estructura:
  HTTP solo en `internal/client`); `limit=1000` minimiza peticiones y cupo (§2, §5). Go 1.25
  permite funciones genéricas; no se pueden usar métodos genéricos sobre `*Client`, de ahí la
  función libre.
- **Alternatives considered**: paginación en cada data source (duplicación); iterador
  perezoso (innecesario: Terraform necesita la lista completa en estado); pedir sin `limit`
  (tamaño por defecto 200 → 5× más peticiones).

## R2. Detección de API 2.x (FR-016)

- **Decision**: doble barrera.
  1. `resolveAPIBase` ya consulta `/health`; se amplía para leer `{"version": "X.Y.Z"}`
     (presente en 2.x y 3.x, verificado en `main` y en la rama 3.0 de IPzilon). Si la versión es
     semver válida y `MAJOR < 3`, `Configure` del provider devuelve un error:
     *"IPzilon X.Y.Z is not supported by this provider version: requires IPzilon >= 3.0.0. Pin the
     provider to ~> 2.2 to keep using IPzilon 2.x."*
  2. Si la versión no es interpretable (p. ej. `0.0.0-dev`, o la sonda falla), no se bloquea;
     `GetAll` detecta una respuesta que empieza por `[` (lista desnuda) y devuelve el mismo
     mensaje de versión mínima en lugar de `cannot unmarshal array`.
- **Rationale**: falla pronto y con mensaje accionable sin romper entornos de desarrollo con
  versión no numérica. Reutiliza la sonda existente: 0 peticiones adicionales.
- **Alternatives considered**: solo detectar en listados (el alta de `ipzilon_ip_address`
  contra 2.x devolvería un 409 engañoso "already in use"); atributo `api_version` en el
  provider (config innecesaria); no detectar (error opaco).

## R3. Reintentos 429/503 (§5)

- **Decision**: bucle de reintento propio dentro de `Client.do()`, sin dependencia nueva.
  - Reintenta **solo** 429 y 503 mientras la espera acumulada de esa petición no supere
    **10 min** (`retryMaxElapsed`), con un máximo de **30 reintentos** (`retryMax`) como red de
    seguridad. Motivo: con un cupo bajo y `-parallelism=10`, varias peticiones compiten por cada
    hueco de la ventana deslizante y un tope de 5 reintentos podría agotarse por pura
    competencia (SC-003).
  - Espera = `Retry-After` (segundos enteros; también se acepta fecha HTTP) si existe; si no,
    backoff exponencial `1s, 2s, 4s, 8s, 16s` con jitter ±20 %. Cada espera se acota a **60 s**.
  - El cuerpo de la petición se serializa una vez y se recrea el `bytes.Reader` en cada
    intento.
  - Al agotarse: `APIError` con el último código y mensaje, más el sufijo
    `(gave up after N retries in <duración>)`.
  - Espera interrumpible: `do()` acepta `context.Context` (ver R6); si el contexto se cancela
    (Ctrl-C en Terraform), se aborta la espera.
  - Cada reintento se registra con `tflog.Debug` (`"retrying IPzilon request"`, con método,
    ruta, código, intento y espera; nunca cabeceras ni token).
  - Tiempos inyectables en el `Client` (`retryMax`, `retryMaxElapsed`, `retryBaseWait`, `retryMaxWait`, `sleep`)
    para tests unitarios rápidos.
- **Rationale**: 429/503 implican que la API no ha procesado la petición (§5), así que
  reintentar un `POST` es seguro. Cubre SC-003: con 10 min de margen por petición, una
  petición que pierde varias veces la competencia por la ventana de 1 min acaba entrando. Evitar `go-retryablehttp` reduce
  superficie de dependencias (Dependabot/CVE) para ~40 líneas de lógica testeable.
- **Alternatives considered**: `hashicorp/go-retryablehttp` (válido, pero reintenta también
  5xx de conexión y requiere configurar `CheckRetry`/`Backoff` a medida; más dependencia que
  beneficio); reintentos configurables en el provider (descartado en la spec, Assumptions).

## R4. Alta de `ipzilon_ip_address` (§3)

- **Decision**: `Create` hace un único `POST /subnets/{subnet_id}/ips` con
  `IPAddressRegister{Address, Status, Hostname, Description}` (`status` por defecto `used`; la
  API por defecto usa `available`, por eso se envía siempre). Mapeo de errores:
  | Respuesta | Diagnóstico |
  |---|---|
  | 201 | Estado desde la respuesta (`id` obligatorio no nulo) |
  | 409 | `IP address in use` — "Address X is already in use in subnet N." |
  | 400 fuera de rango (`is not within subnet`) | `IP address outside subnet` — "Address X is not within subnet N." |
  | 400 otro | `Create IP failed` con el mensaje de la API |
  | 404 | `Subnet not found` |
- **Verificado** en `backend/app/routers/subnets.py:293` (`register_ip`): cuerpo
  `IPAddressBase` (`address`, `status`, `hostname`, `description`, `owner`, `mac_address`,
  `notes`), 409 `"IP … is already in use in this subnet"`, 400 `"IP … is not within subnet …"`.
- **Read/Update**: sin cambios de lógica.
- **Delete** (aplica también a `ipzilon_next_ip_address`): pasa de `PATCH` a `available` a
  `DELETE /ips/{id}` (204). En 3.0 `DELETE` **libera** la dirección (borra su registro y vuelve a
  quedar libre); en 2.x la eliminaba de la subred, motivo por el que el provider usaba `PATCH`,
  pero la v3.0.0 no soporta 2.x. Un 404 se ignora (ya liberada). Las reservadas de Azure dan 403
  igual con `DELETE` que con `PATCH` (verificado en `routers/ip_addresses.py`).
- **Schema**: `address` añade validador de formato IP (Principio I, validación de forma) y
  cambia su `Description` (ya no "pre-populated"). `status` pasa a
  `stringvalidator.OneOf("used", "reserved")` en `ipzilon_ip_address` e
  `ipzilon_next_ip_address` (R11).

## R5. `IPAddress.ID` nulable

- **Decision**: `client.IPAddress.ID` pasa a `*int64`. Consumidores:
  - `datasources/ip_addresses.go` → `types.Int64PointerValue(ip.ID)` (nulo en libres).
  - `resources/ip_address.go`, `next_ip_address.go` → helper `ipIDValue` que exige no nulo
    tras `POST`/`GET /ips/{id}`; si viniera nulo, error `Unexpected response: IP address
    without id` (no debería ocurrir en rutas con fila).
  - `next_ip_address.go` usa `*ip.ID` para el `PATCH` posterior a `reserve-ip`.
- **Rationale**: evita el `id = 0` falso (§9.1). El atributo `items[*].id` ya es `Computed`:
  pasar a nulo no cambia el esquema (sin `UpgradeState`).

## R6. `context.Context` en el cliente

- **Decision**: cambiar la firma interna a `do(ctx, …)` y las públicas a
  `Get(ctx, path, out)`, `Post(ctx, …)`, `Patch(ctx, …)`, `Delete(ctx, …)` y
  `GetAll[T](ctx, c, path)`. Todos los llamadores tienen `ctx` disponible.
- **Rationale**: necesario para cancelar esperas de reintento (R3) y buena práctica en
  `terraform-plugin-framework`. Cambio mecánico en ~60 llamadas, sin efecto para el usuario.
- **Alternatives considered**: `context.Background()` en `do` (las esperas de hasta 60 s no
  se podrían cancelar).

## R7. Búsqueda truncada (§6)

- **Decision**: helper `client.IsSearchTruncated(err)` = `APIError` con `Code == 409` y
  mensaje que empieza por `"Search truncated"` (texto verificado en
  `routers/networks.py:282,326` y `routers/scopes.py:356`). En `next_subnet`, `last_subnet` y
  `next_network`, diagnóstico `Address space too fragmented` con detalle: *"IPzilon stopped
  searching for a free /N block because the space is too fragmented. Declare an explicit CIDR
  with ipzilon_subnet / ipzilon_network instead. API: …"*. El 409 `"No free /N block
  available"` conserva el diagnóstico actual.
- **Alternatives considered**: comparar el mensaje completo (frágil ante cambios de texto
  tras el prefijo).

## R8. Validación /8 e IPv6 en `plan` (§6)

- **Decision**: validadores de atributo en `internal/resources`:
  - `cidrMaxSize(8)`: si el valor es un CIDR IPv4 válido con prefijo < 8 → error
    *"/N is too large: the maximum size is /8"*. Aplicado a `ipzilon_hub.address_space`,
    `ipzilon_scope.cidr`, `ipzilon_network.cidr`, `ipzilon_subnet.cidr`.
  - `cidrIPv4Only()`: en `ipzilon_subnet.cidr`, error *"IPv6 subnets are not supported"*.
  - Valores desconocidos o nulos se ignoran; CIDR no parseable se deja a la API (no se
    añade validación nueva de formato para no cambiar comportamiento).
- **Objetos existentes > /8** (FR-014, escenario US5-3): un validador de atributo se ejecuta
  sobre la configuración sin mirar el estado, así que rompería el `plan` de un hub /7 heredado
  cuya configuración declara ese mismo valor. Para no bloquear su gestión, se implementa como
  **plan modifier / `ModifyPlan`** que solo valida cuando el valor planificado **difiere** del
  estado (creación o cambio), igual que la API (§6: el tope se aplica al crear o cambiar).
- **Rationale**: falla en `plan` sin impedir el `plan` de objetos heredados.
- **Alternatives considered**: validador simple de atributo (rompería el `plan` de hubs
  existentes > /8 importados).

## R9. Versionado y compatibilidad de estado

- **Decision**: release `v3.0.0` del provider. Sin cambios de esquema incompatibles:
  ningún atributo renombrado, eliminado ni convertido en obligatorio → no hace falta
  `UpgradeState` (Principio III). El salto MAJOR se debe al fin de soporte de IPzilon 2.x.
  `README.md`: matriz de compatibilidad y *Breaking Changes*. Ejemplos y `docs/index.md`:
  `~> 3.0`.
- **Estado existente**: los `id` de direcciones ocupadas se conservan tras la migración de
  IPzilon (§9.2, SC-009 del informe) → SC-004 se cumple sin cambios en `Read`.

## R10. Coste de `ipzilon_ip_addresses` sin `status` (FR-007)

- **Decision**: solo documentación (aclaración Q1 = A): `Description` del atributo `status` y
  del data source, más nota en el ejemplo. Una /16 = 66 peticiones con `limit=1000`.

## R11. `status` sin `available` en recursos de dirección (FR-020)

- **Decision**: `status` solo admite `used` o `reserved` (validador en `plan`). Liberar = destruir
  el recurso; bloquear una IP = `reserved`.
- **Rationale**: en 3.0, `available` sin anotaciones no tiene registro: el alta falla (400) y, al
  modificar, IPzilon libera la IP y Terraform queda con un `id` inexistente → bucle de
  recreación. `available` con anotaciones tiene sentido en la UI, no en un recurso de Terraform
  que representa una dirección ocupada.
- **Alternatives considered**: exigir `hostname`/`description` cuando `status = "available"`
  (regla poco evidente y sin caso de uso real en Terraform).
- **Impacto**: cambio incompatible del esquema (se estrechan los valores admitidos); va en
  *Breaking Changes* de v3.0.0. Sin `UpgradeState`: un estado con `available` hará fallar el
  `plan` con un mensaje que indica cambiar a `reserved`/`used` o destruir el recurso.

## Fuera de alcance confirmado

Bulk de direcciones, listados no consumidos, `owner`/`mac_address`/`notes` (no están en el
esquema actual; añadirlos sería una feature aparte), campo `truncated` de
`GET /hubs/{id}/next-available-network` (el provider no lo consume).
