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

output "managed_identity_client_id" {
  description = "Client ID of the app's user-assigned identity (the Key Vault reader)."
  value       = azurerm_user_assigned_identity.app.client_id
}
