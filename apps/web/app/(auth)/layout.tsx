import Link from "next/link";
import { Logo } from "@/components/brand/logo";

export default function AuthLayout({ children }: LayoutProps<"/">) {
  return (
    <main
      id="main"
      tabIndex={-1}
      className="flex flex-1 flex-col items-center justify-center gap-8 px-4 py-16 outline-none"
    >
      <Link href="/" aria-label="KubeDojo home" className="rounded-md">
        <Logo />
      </Link>
      {children}
    </main>
  );
}
