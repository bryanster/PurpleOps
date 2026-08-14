output "app_url" {
  description = "The HTTPS URL of the deployed Blacklight app."
  value       = "https://${local.container_app_name}.${azurerm_container_app_environment.main.default_domain}"
}

output "resource_group_name" {
  description = "Resource group holding the deployment."
  value       = azurerm_resource_group.main.name
}

output "container_app_name" {
  description = "Container App name (used for `az containerapp` commands)."
  value       = azurerm_container_app.main.name
}

output "key_vault_name" {
  description = "Key Vault holding the generated session secret and encryption key."
  value       = azurerm_key_vault.main.name
}

output "storage_account_name" {
  description = "Storage account holding the Azure Files share with the database and evidence."
  value       = azurerm_storage_account.main.name
}

output "admin_email" {
  description = "Address the first administrator signs in with, or null when this deployment creates no account."
  value       = var.admin_email
}

output "admin_password_command" {
  description = "Command that prints the first administrator's initial password. The value is write-only, so Key Vault is the only place it exists — Terraform cannot show it here. Null when no account is created."

  # Built from the vault and secret *names*, never from the secret resource
  # itself: every attribute of an azurerm_key_vault_secret is sensitive, so a
  # reference to one would make this whole string sensitive and unprintable.
  value = local.create_admin ? join(" ", [
    "az keyvault secret show",
    "--vault-name ${azurerm_key_vault.main.name}",
    "--name ${local.admin_secret_name}",
    "--query value -o tsv",
  ]) : null

  # The command is not the password — it is what an operator runs to fetch it
  # from Key Vault, having already authenticated to Azure.
  depends_on = [azurerm_key_vault_secret.admin_password]
}

output "managed_identity_client_id" {
  description = "Client ID of the app's user-assigned identity (the Key Vault reader)."
  value       = azurerm_user_assigned_identity.app.client_id
}
