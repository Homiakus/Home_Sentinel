# Home Assistant integration assets

Sentinel entities are created through MQTT Device Discovery. These files are optional for Home Assistant installations that deliberately use file-managed packages/YAML-mode Lovelace dashboards.

Sentinel never writes Home Assistant `.storage` files and never emulates undocumented config-flow requests. Configure the MQTT and Frigate integrations through Home Assistant's supported UI/config flow, then use Sentinel verification to confirm them.
