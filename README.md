> [!WARNING]  
> This project is a work in progress and has not been battle-tested yet.

![BOTTLED_WATER.webp](./BOTTLED_WATER.webp)

<sub><sup>logo from OMORI</sup></sub>

# water
Simple upgrade tool made for [bouquet2](https://github.com/bouquet2/bouquet2)

## Features
* Minimalistic
* Declarative configuration through YAML
* Safe upgrades
  * Upgrades control-plane first, workers last
    * Adjustable through configuration
* Support for Talos and Kubernetes versions
* Version checking
  * Checks repositories of Kubernetes and Talos to make sure you're not trying to upgrade to a version that doesn't exist yet
  * Only performs upgrades when current versions don't match target versions
* Dry run mode 

## Installation

### Binary Releases
Download the latest binary for your platform from the [releases page](https://github.com/bouquet2/water/releases).

## Configuration

Water uses a YAML configuration file to specify the desired versions for your cluster:

```yaml
talos:
  imageId: "factory.talos.dev/installer/8cdf4cd0a3a9fa4771aab65437032804940f2115b1b1ef6872274dde261fa319"
  version: "v1.10.5"
  upgradeOrder: "control-plane-first"  # Optional: "control-plane-first" or "workers-first"
k8s:
  version: "v1.33.3"
  upgradeOrder: "workers-first"        # Optional: "control-plane-first" or "workers-first"
```

### Configuration Fields

- `talos.imageId`: The Talos image ID to upgrade to
- `talos.version`: The target Talos version (must start with 'v')
- `talos.upgradeOrder`: Optional. Order for Talos node upgrades: `"control-plane-first"` (default) or `"workers-first"`
- `k8s.version`: The target Kubernetes version (must start with 'v')
- `k8s.upgradeOrder`: Optional. Order for Kubernetes node upgrades: `"control-plane-first"` (default) or `"workers-first"`

### Upgrade Order Options

You can control the order in which nodes are upgraded for both Talos and Kubernetes separately:

- **`control-plane-first`** (default): Upgrades control plane nodes first, then worker nodes. This is the traditional and safer approach.
- **`workers-first`**: Upgrades worker nodes first, then control plane nodes. This can be useful in certain scenarios where you want to test the upgrade on workers first.

## License
water is free software: you can redistribute it and/or modify it under the terms of the GNU Affero General Public License as published by the Free Software Foundation, either version 3 of the License.

water is distributed in the hope that it will be useful, but WITHOUT ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License along with water. If not, see https://www.gnu.org/licenses/.
