import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Atur ulang kata sandi",
  robots: {
    index: false,
    follow: false,
  },
};

export default function ResetPasswordLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return children;
}
