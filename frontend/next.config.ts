import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  devIndicators: false,
  // Menghasilkan runtime Node yang ringkas untuk systemd/container tanpa
  // membawa seluruh node_modules ke server production.
  output: "standalone",
};

export default nextConfig;
