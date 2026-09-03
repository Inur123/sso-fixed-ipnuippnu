import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  devIndicators: false,
  // Menghasilkan runtime Node yang ringkas untuk systemd/container tanpa
  // membawa seluruh node_modules ke server production.
  output: "standalone",
  async headers() {
    return [
      {
        source: "/reset-password",
        headers: [
          { key: "Cache-Control", value: "no-store" },
          { key: "Referrer-Policy", value: "no-referrer" },
          { key: "X-Robots-Tag", value: "noindex, nofollow" },
        ],
      },
    ];
  },
};

export default nextConfig;
