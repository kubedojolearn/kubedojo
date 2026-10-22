import type { Metadata } from "next";
import { Container, PageHeader } from "@/components/layout/page-header";
import { Prose } from "@/components/layout/prose";
import { site } from "@/lib/site";

export const metadata: Metadata = {
  title: "Privacy policy",
  description: "What KubeDojo collects, why, how long it is kept, and how to export or delete it.",
};

export default function PrivacyPage() {
  return (
    <Container className="py-12 sm:py-16">
      <PageHeader title="Privacy policy" description="Last updated 22 October 2026" />
      <Prose>
        <p>
          KubeDojo is in development and does not yet create accounts. This policy describes how KubeDojo will handle
          your data when sign-in opens. We&apos;ll update the date above whenever it changes.
        </p>

        <h2>The short version</h2>
        <ul>
          <li>We collect the minimum needed to save your progress.</li>
          <li>We never ask for your email address.</li>
          <li>We don&apos;t sell data, show ads or use third-party trackers.</li>
          <li>You can export or delete your data at any time from your settings.</li>
        </ul>

        <h2>What we collect</h2>
        <table>
          <thead>
            <tr>
              <th scope="col">Data</th>
              <th scope="col">Why</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td>GitHub account ID, username, display name and avatar URL</td>
              <td>To sign you in and show your profile. The numeric ID identifies you; the rest is for display.</td>
            </tr>
            <tr>
              <td>Lesson progress, XP, streaks and achievements</td>
              <td>To show your progress and, only if you opt in, rank you on the leaderboard.</td>
            </tr>
            <tr>
              <td>Your lab sessions: simulated cluster state, files you edit, commands you run, answers you give</td>
              <td>To save and resume your labs and to grade your work.</td>
            </tr>
            <tr>
              <td>Settings such as theme, reduced motion, time zone and privacy choices</td>
              <td>To apply your preferences.</td>
            </tr>
            <tr>
              <td>Learning activity events: which lesson you started or finished, and when</td>
              <td>
                To improve lessons that people find hard. These events never include the commands, YAML or answers you
                type. You can turn them off in settings.
              </td>
            </tr>
          </tbody>
        </table>

        <h2>Cookies and local storage</h2>
        <p>
          We use one essential, HttpOnly session cookie to keep you signed in. Your theme choice is stored in your
          browser&apos;s local storage. There are no advertising or analytics cookies.
        </p>

        <h2>Simulated Secrets</h2>
        <p>
          Some lessons ask you to create Kubernetes Secrets. Use the training values from the lesson, never real
          passwords or keys. Secret values are excluded from our logs and analytics, and deleted with the rest of your
          lab data.
        </p>

        <h2>How long we keep it</h2>
        <table>
          <thead>
            <tr>
              <th scope="col">Data</th>
              <th scope="col">Kept for</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td>Account, profile, settings, progress, XP and achievements</td>
              <td>Until you delete your account</td>
            </tr>
            <tr>
              <td>Lab contents (cluster state, files, commands, answers)</td>
              <td>
                Active labs expire after 14 days without activity; contents are deleted 30 days after a lab ends.
                Command history is never kept longer than 90 days.
              </td>
            </tr>
            <tr>
              <td>Learning activity events</td>
              <td>180 days</td>
            </tr>
            <tr>
              <td>Server logs</td>
              <td>30 days</td>
            </tr>
            <tr>
              <td>Backups</td>
              <td>30 days, rolling</td>
            </tr>
          </tbody>
        </table>

        <h2>Your choices and rights</h2>
        <ul>
          <li>
            <strong>Export:</strong> download all your data as JSON from settings.
          </li>
          <li>
            <strong>Delete:</strong> delete your account from settings. Your profile disappears immediately and the rest
            of your personal data is erased within 30 days, including from backups. Signing in again later creates a
            new, empty account.
          </li>
          <li>
            <strong>Privacy controls:</strong> your profile is private and you are off the leaderboard unless you choose
            otherwise.
          </li>
        </ul>

        <h2>Who processes your data</h2>
        <p>
          GitHub processes your sign-in under its own privacy policy. KubeDojo&apos;s hosting provider stores our
          database and logs; it will be named here before launch.
        </p>

        <h2>Contact</h2>
        <p>
          Questions about privacy can be raised in the <a href={site.repoUrl}>GitHub repository</a>. To report a
          security issue privately, use <a href={site.securityUrl}>GitHub&apos;s vulnerability reporting</a>.
        </p>
      </Prose>
    </Container>
  );
}
