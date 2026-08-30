# Deploy to Azure (one click)

[![Deploy to Azure](https://aka.ms/deploytoazurebutton)](https://portal.azure.com/#create/Microsoft.Template/uri/https%3A%2F%2Fraw.githubusercontent.com%2Fbryanster%2Fblacklight%2Fmain%2Fdeploy%2Fazure-arm%2Fazuredeploy.json)

The button opens the Azure portal on `azuredeploy.json` in this directory — an ARM template that
provisions the same deployment as [`deploy/azure-terraform/`](../azure-terraform/README.md), without
a Terraform install or a state file to keep. Pick a subscription, a resource group and a region, and
optionally the address of the first administrator; everything else has a working default.

What it provisions:

| Resource | Purpose |
|---|---|
| Container App (single replica) | Runs the published `ghcr.io/bryanster/blacklight` image |
| Storage Account + Azure Files share | Persistent `/var/lib/blacklight` — the DuckDB database, its WAL and uploaded evidence |
| Key Vault | Holds `BLACKLIGHT_SESSION_SECRET`, `BLACKLIGHT_ENCRYPTION_KEY` and — if you ask for a first administrator — that account's initial password |
| User-assigned managed identity | Lets the app read the Key Vault secrets. It is the only principal the template grants data-plane access to |
| Log Analytics workspace + Container Apps environment | Required base for Container Apps |

The two application secrets are generated during the deployment, stored in Key Vault, and referenced
by the container app through the managed identity — the values never appear in the container app's
own secret list. They are `securestring` parameters, so they are not recorded in the deployment
history either; Key Vault is where they live.

## Prerequisites

- An Azure subscription where you can create resource groups, Key Vaults and Container Apps
  (Owner, or Contributor — the template grants Key Vault access with an access policy, not a role
  assignment, so no User Access Administrator rights are needed).
- The `Microsoft.App`, `Microsoft.OperationalInsights`, `Microsoft.Storage`, `Microsoft.KeyVault` and
  `Microsoft.ManagedIdentity` resource providers registered. The portal registers them for you if
  your account may; to do it up front:

  ```sh
  for ns in Microsoft.App Microsoft.OperationalInsights Microsoft.Storage Microsoft.KeyVault Microsoft.ManagedIdentity; do
    az provider register --namespace "$ns"
  done
  ```

Deployment takes about five minutes, most of it the Container Apps environment.

## Parameters

Everything is optional; these are the defaults.

| Parameter | Default | What it does |
|---|---|---|
| `name` | `blacklight` | Names the Container App, environment, workspace and identity, and the first label of the app's hostname |
| `location` | the resource group's region | Region for every resource |
| `image` / `imageTag` | `ghcr.io/bryanster/blacklight` / `v1.0.2` | The image to run. Pin a release |
| `containerCpu` / `containerMemory` | `1` / `2Gi` | Container size. Must be a valid Consumption-plan pair |
| `storageShareQuotaGb` | `100` | Size of the Azure Files share |
| `adminEmail` | *(empty)* | Set it and the app creates that first administrator; leave it empty and no account is created |
| `adminName` | `Administrator` | Display name for that account |
| `createSecrets` | `true` | Whether to write the three Key Vault secrets. **Set it to `false` when redeploying** — see below |
| `sessionSecret` / `encryptionKey` / `adminPassword` | generated | Supply your own instead of the generated values |

The Storage Account and Key Vault take a generated name (`st…`/`kv…` plus a hash of the resource
group and `name`), because both must be globally unique.

## After it deploys

The **Outputs** tab of the deployment gives you `appUrl`, the Key Vault and storage account names,
and `adminPasswordCommand`.

A fresh deployment has no users and no sign-up (see [`docs/deploy.md`](../../docs/deploy.md)), so
something has to create the first account.

### The first administrator, from the template

Set `adminEmail` in the portal form. The template generates a password, stores it in Key Vault beside
the other two secrets, and passes the address, the name and the password to the app. The app creates
that account — as a platform administrator, active, with a local password — **the first time it
starts against a database with no accounts in it**, and ignores the setting on every start
afterwards. Read the password out of Key Vault and sign in:

```sh
az keyvault secret show --vault-name <keyVaultName from the outputs> \
  --name blacklight-bootstrap-admin-password --query value -o tsv
```

Change the password at first sign-in. Two things worth being clear about, both of which apply
equally to the Terraform version:

- **It is a bootstrap, not an account manager.** The app acts on those variables only when there are
  no accounts at all. It cannot reset a password, re-promote an account somebody demoted, or revive
  one somebody disabled — which is what makes it safe to leave `adminEmail` set for the life of the
  deployment. A genuinely forgotten password is a `blctl` job.
- **The password reaches the app as an environment variable.** The revision holds a *reference* to
  the Key Vault secret rather than the value, but the value ends up in the replica's process
  environment. The application also accepts `BLACKLIGHT_BOOTSTRAP_ADMIN_PASSWORD_FILE`, which is the
  better of the two, but Container Apps volumes take only Azure Files and EmptyDir here, so a secret
  cannot be mounted as a file. Change the password at first sign-in, and clear `adminEmail` and
  redeploy if you want the variable off the revision entirely.

### The first administrator, with `blctl`

The other way, and the only one on a deployment you would rather not hand a password to. `blctl user
create` needs exclusive access to the database — DuckDB gives the file to one process at a time — so
it means stopping the app, running `blctl` once against the same share, and starting it again. The
[Terraform README](../azure-terraform/README.md#with-blctl) has the commands; they are identical
here, with the resource group being the one you chose in the portal.

## Redeploying

**Set `createSecrets` to `false` on any deployment after the first.** The generated defaults are
regenerated every time the template runs, so a redeploy that leaves it `true` writes new values into
Key Vault: a new session secret signs every user out, and a new encryption key makes every enrolled
MFA authenticator, recovery code and service token unreadable. With `createSecrets: false` the
existing Key Vault secrets are left untouched and everything else — image tag, container size, share
quota — still updates. The container app references the secrets by their versionless Key Vault URL,
so it keeps resolving them.

Upgrading the image, from the CLI:

```sh
az deployment group create \
  --resource-group <your-rg> \
  --template-file deploy/azure-arm/azuredeploy.json \
  --parameters @deploy/azure-arm/azuredeploy.parameters.json \
  --parameters createSecrets=false imageTag=v1.0.3
```

`az containerapp update --image ghcr.io/bryanster/blacklight:v1.0.3` does the same thing for that one
field, and cannot touch the secrets at all.

To rotate a secret deliberately, write the new value to Key Vault and restart the revision — Container
Apps caches Key Vault references:

```sh
az keyvault secret set --vault-name <kv> --name blacklight-session-secret --value "$(openssl rand -hex 32)"
az containerapp revision restart --name <app> --resource-group <your-rg> --revision "$(az containerapp show --name <app> --resource-group <your-rg> --query properties.latestRevisionName -o tsv)"
```

Rotating the encryption key is destructive in the way described above. Only do it as part of a
deliberate re-enrollment.

## Backups

`blctl backup` and `blctl migrate` have the same single-process constraint as `blctl user create`:
stop the app, run the command against the same share, start it again. Back up the Key Vault secrets
too — the session secret and encryption key live there and nowhere else, and a restored database
without its encryption key is one where nobody with MFA can sign in.

## Caveats

Everything in the Terraform README's [caveats](../azure-terraform/README.md#caveats) applies —
single replica is load-bearing, DuckDB on SMB, `/dev/shm` as an EmptyDir mount, no scale-to-zero —
plus two that are specific to this template:

- **Secrets are template inputs, not write-only values.** Terraform generates them with ephemeral
  resources and writes them with `value_wo`, so they never reach state. ARM has no equivalent: the
  values are `securestring` parameters, which keeps them out of the deployment history, but they do
  pass through the deployment. `createSecrets` is what stops a redeploy from overwriting them.
- **Deleting and redeploying into the same resource group collides with the soft-deleted Key
  Vault.** The vault name is derived from the resource group and `name` rather than being random, so
  it is the same name on the second deployment while the first is still in Key Vault's 7-day soft-delete
  window. Either purge it (`az keyvault purge --name <kv> --location <region>`) or deploy with a
  different `name`.

## Teardown

```sh
az group delete --name <your-rg>
```

The Key Vault is soft-deleted rather than purged; see the caveat above before redeploying with the
same resource group and `name`.

## Which one should I use?

This template for a first deployment, a demo, or an environment nobody is going to change often —
one click, nothing to install, no state to store. [`deploy/azure-terraform/`](../azure-terraform/)
for anything with a lifecycle: it keeps the secrets out of the deployment entirely (ephemeral values
and write-only arguments), makes rotation an explicit counter, and can plan a change before making
it. Both produce the same resources, so a deployment made with the button can be adopted by
Terraform later with `terraform import` — pass the existing secrets back in with
`TF_VAR_session_secret` and `TF_VAR_encryption_key` so the adoption does not rewrite them.
