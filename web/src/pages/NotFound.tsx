import { Link } from 'react-router'
import Page from './Page.tsx'

export default function NotFound() {
  return (
    <Page title="Page not found">
      <Link to="/" className="text-sky-600 underline dark:text-sky-400">
        Back to the overview
      </Link>
    </Page>
  )
}
