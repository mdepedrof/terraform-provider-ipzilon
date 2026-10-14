mock_provider "ipzilon" {
  mock_data "ipzilon_hubs" {
    defaults = { items = [{ id = 7 }] }
  }

  mock_data "ipzilon_ip_addresses" {
    defaults = { items = [{ id = 7 }] }
  }

  mock_data "ipzilon_network_zones" {
    defaults = { items = [{ id = 7 }] }
  }

  mock_data "ipzilon_networks" {
    defaults = { items = [{ id = 7 }] }
  }

  mock_data "ipzilon_scopes" {
    defaults = { items = [{ id = 7 }] }
  }

  mock_data "ipzilon_sites" {
    defaults = { items = [{ id = 7 }] }
  }

  mock_data "ipzilon_subnets" {
    defaults = { items = [{ id = 7 }] }
  }
}

run "mock_data_defaults" {
  command = plan

  assert {
    condition     = output.hub == 7
    error_message = "ipzilon_hubs: items[0].id should be 7"
  }

  assert {
    condition = alltrue([
      output.ip_addresses_count == 1 && output.ip_addresses_ids == [7],
      output.network_zones_count == 1 && output.network_zones_ids == [7],
      output.networks_count == 1 && output.networks_ids == [7],
      output.scopes_count == 1 && output.scopes_ids == [7],
      output.sites_count == 1 && output.sites_ids == [7],
      output.subnets_count == 1 && output.subnets_ids == [7],
    ])
    error_message = "every list data source should return one item with id 7"
  }
}
