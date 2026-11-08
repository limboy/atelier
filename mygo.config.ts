import { defineConfig } from "mygo-cli";
import pkg from "./package.json" with { type: "json" };

export default defineConfig({
  name: "Atelier",
  identifier: "me.limboy.atelier",
  version: pkg.version,
  // Installed apps check the latest GitHub release and accept only updates
  // signed with the secret key that matches this public key.
  updates: {
    publicKey: "D1aycXnFSIS9EFYakeFrjqYk+4k0K0W42vTEjJ6eHEI=",
    github: "limboy/artsandculture",
  },
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
