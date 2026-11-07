import { defineConfig } from "mygo-cli";

export default defineConfig({
  name: "Atelier",
  identifier: "me.limboy.atelier",
  version: "0.1.0",
  // Native UI only: no frontend to start or embed. `mygo dev` rebuilds and
  // restarts the app as the Go code, this file or resources/ change.
  out: "build",
  macos: {
    signingIdentity: process.env.APPLE_SIGNING_IDENTITY,
    notarize: process.env.APPLE_NOTARY_PROFILE
      ? {
          keychainProfile: process.env.APPLE_NOTARY_PROFILE,
          keychain: process.env.APPLE_KEYCHAIN_PATH,
        }
      : undefined,
  },
});
