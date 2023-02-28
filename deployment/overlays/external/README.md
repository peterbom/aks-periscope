# External Overlay

This overlay produces the Periscope resource specification for the production images in MCR, and is used to produce the assets that are published for Periscope releases.

Configuration data like storage details and run ID is not known at this time. Consumers are responsible for substituting all configuration data into the output, so we produce well-known placeholders for the various settings.

```sh
# Important: set the desired MCR version tag
image_tag=...

# Output base and feature resource specifications to ./publish directory
./deployment/overlays/external/build-yaml.sh $image_tag base
./deployment/overlays/external/build-yaml.sh $image_tag win-hpc
```