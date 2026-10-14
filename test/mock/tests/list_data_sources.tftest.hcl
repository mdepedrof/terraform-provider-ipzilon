mock_provider "ipzilon" {}

# Issue #26: a list with a partial object must reach the expressions.
override_data {
  target = data.ipzilon_hubs.regional
  values = { items = [{ id = 1 }] }
}

override_data {
  target = data.ipzilon_ip_addresses.gw
  values = { items = [{ id = 11 }] }
}

override_data {
  target = data.ipzilon_network_zones.pooled
  values = { items = [{ id = 12 }] }
}

override_data {
  target = data.ipzilon_networks.avd
  values = { items = [{ id = 13 }] }
}

override_data {
  target = data.ipzilon_scopes.avd
  values = { items = [{ id = 14 }] }
}

override_data {
  target = data.ipzilon_sites.all
  values = { items = [{ id = 15 }] }
}

override_data {
  target = data.ipzilon_subnets.all
  values = { items = [{ id = 16 }] }
}

run "one_partial_item" {
  command = plan

  assert {
    condition     = output.hub == 1
    error_message = "ipzilon_hubs: items[0].id should be 1"
  }

  assert {
    condition     = output.ip_addresses_count == 1 && output.ip_addresses_ids == [11]
    error_message = "ipzilon_ip_addresses: expected one item with id 11"
  }

  assert {
    condition     = output.network_zones_count == 1 && output.network_zones_ids == [12]
    error_message = "ipzilon_network_zones: expected one item with id 12"
  }

  assert {
    condition     = output.networks_count == 1 && output.networks_ids == [13]
    error_message = "ipzilon_networks: expected one item with id 13"
  }

  assert {
    condition     = output.networks_first_name != null
    error_message = "ipzilon_networks: a field missing from the mock should get a generated value"
  }

  assert {
    condition     = output.scopes_count == 1 && output.scopes_ids == [14]
    error_message = "ipzilon_scopes: expected one item with id 14"
  }

  assert {
    condition     = output.sites_count == 1 && output.sites_ids == [15]
    error_message = "ipzilon_sites: expected one item with id 15"
  }

  assert {
    condition     = output.subnets_count == 1 && output.subnets_ids == [16]
    error_message = "ipzilon_subnets: expected one item with id 16"
  }
}

run "several_and_empty" {
  command = plan

  override_data {
    target = data.ipzilon_sites.all
    values = { items = [{ id = 1, name = "a" }, { id = 2, name = "b" }] }
  }

  override_data {
    target = data.ipzilon_subnets.all
    values = { items = [] }
  }

  assert {
    condition     = output.sites_count == 2 && output.sites_ids == [1, 2]
    error_message = "ipzilon_sites: expected two items with ids 1 and 2"
  }

  assert {
    condition     = output.subnets_count == 0
    error_message = "ipzilon_subnets: expected an empty list"
  }
}
