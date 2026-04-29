// Live TV title parts shared between the active-stream card and the history table.
// Channel goes on the primary line; the program (with optional EPG subtitle)
// goes on the secondary line. When EPG data is missing, the adapter sets title
// equal to the channel name and we suppress the redundant subtitle.

export interface LiveTVTitleSource {
  title: string
  parent_title: string
  grandparent_title: string
}

export interface LiveTVTitleParts {
  primary: string
  subtitle: string
  showSubtitle: boolean
}

export function getLiveTVTitleParts(s: LiveTVTitleSource): LiveTVTitleParts {
  const showSubtitle = !!s.title && s.title !== s.grandparent_title
  const subtitle = s.parent_title ? `${s.title} · ${s.parent_title}` : s.title
  return {
    primary: s.grandparent_title || s.title,
    subtitle,
    showSubtitle,
  }
}
