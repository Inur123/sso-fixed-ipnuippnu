import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Daftar",
};

export default function RegisterLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return children;
}
