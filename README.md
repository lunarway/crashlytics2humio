# Crashlytics incidents to Humio

This project forwarads Crashlytics incident notifications to Humio.

This is considered in *alpha* stage. APIs and usage will change.

# Design

Crashlytics can trigger [webhooks on incident impact changes](https://docs.fabric.io/android/crashlytics/custom-web-hooks.html?web%20hooks#custom-web-hooks).
This server will setup an HTTP endpoint for these webhooks and push the data into Humio.
