import type { Metadata } from "next";
import { InDevelopmentBadge } from "@/components/feedback/in-development";
import { Container, PageHeader } from "@/components/layout/page-header";
import { Prose } from "@/components/layout/prose";
import { site } from "@/lib/site";

export const metadata: Metadata = {
  title: "Terms of use",
  description: "The terms for using KubeDojo.",
};

export default function TermsPage() {
  return (
    <Container className="py-12 sm:py-16">
      <PageHeader
        title="Terms of use"
        eyebrow={<InDevelopmentBadge label="Draft" />}
        description="Last updated 22 October 2026. These terms will be reviewed before KubeDojo opens sign-in."
      />
      <Prose>
        <p>
          By using KubeDojo you agree to these terms. They are written to be read; if anything is unclear, ask in the{" "}
          <a href={site.repoUrl}>GitHub repository</a>.
        </p>

        <h2>What KubeDojo is</h2>
        <p>
          KubeDojo is a free, open-source learning service. Labs run against a simulated Kubernetes cluster that
          implements a documented subset of Kubernetes behavior. It is not a real cluster, not a production environment,
          and not a substitute for official Kubernetes documentation. KubeDojo makes no promise that using it will lead
          to any certification.
        </p>

        <h2>Your account</h2>
        <p>
          You sign in with GitHub. Keep your GitHub account secure; you are responsible for activity under it. You can
          delete your KubeDojo account at any time from settings.
        </p>

        <h2>Acceptable use</h2>
        <p>Please don&apos;t:</p>
        <ul>
          <li>try to access other people&apos;s labs, progress or accounts;</li>
          <li>attack, overload or probe the service, except through responsible disclosure;</li>
          <li>use automation to farm XP or manipulate the leaderboard;</li>
          <li>paste real passwords, keys or other secrets into labs;</li>
          <li>use your profile name or avatar to harass or impersonate anyone.</li>
        </ul>
        <p>We may suspend accounts that break these rules.</p>

        <h2>Content and code</h2>
        <p>
          KubeDojo&apos;s source code and lesson content are published under the Apache License 2.0 in the{" "}
          <a href={site.repoUrl}>repository</a>. Contributions are accepted under the same license.
        </p>

        <h2>Availability and changes</h2>
        <p>
          KubeDojo is provided as is, without warranty, and may change, pause or stop. We&apos;ll give notice in the
          changelog before removing features that hold your data, and you can export your data at any time.
        </p>

        <h2>Liability</h2>
        <p>
          To the extent the law allows, KubeDojo&apos;s maintainers are not liable for losses arising from use of the
          service. Nothing in these terms limits rights you have under consumer law that cannot be limited.
        </p>

        <h2>Changes to these terms</h2>
        <p>We&apos;ll post changes here and in the changelog, with the date they take effect.</p>
      </Prose>
    </Container>
  );
}
