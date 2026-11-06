// The made-up server the prototype shows: five XFS data disks and a btrfs
// cache pool, a `data/{media,torrents,usenet}` share with hardlinks, Photos,
// backups that come and go, app data and VMs on the cache, and some 14
// months of History. Every name is invented. The same seed always builds the
// same server, relative to today.

import { addDays, dayOf } from '../../lib/days.ts'
import type { FakeFile, FakeWorld } from './fake.ts'

const KiB = 1024
const MiB = 1024 * KiB
const GiB = 1024 * MiB
const TB = 1e12

/** Days of History before today. */
const historyDays = 430

export function makeWorld(now: Date): FakeWorld {
  const random = mulberry32(20261106)
  const today = dayOf(now)
  const firstDay = addDays(today, -historyDays)

  const between = (lo: number, hi: number) => lo + random() * (hi - lo)
  const int = (lo: number, hi: number) => Math.floor(between(lo, hi + 1))
  const pick = <T>(items: readonly T[]): T => items[Math.floor(random() * items.length)]
  const chance = (p: number) => random() < p
  /** A day from long before the first Scan up to today; later days get likelier with `lean`. */
  const someDay = (pBefore: number, lean = 1.6) =>
    chance(pBefore)
      ? addDays(firstDay, -int(1, 2500))
      : addDays(firstDay, Math.floor(historyDays * random() ** (1 / lean)))
  const dayWithin = (from: string, maxDays: number) => {
    const day = addDays(from, int(0, maxDays))
    return day > today ? null : day
  }

  // Disks: disk5 was added to the array 180 days ago and only fills from then.
  // New files go to a disk at random, likelier on the first disks and on
  // ones with room, so the disks fill unevenly, as Unraid's allocation does.
  const capacity: Record<string, number> = {
    disk1: 10 * TB,
    disk2: 10 * TB,
    disk3: 10 * TB,
    disk4: 10 * TB,
    disk5: 14 * TB,
  }
  const filled: Record<string, number> = { disk1: 0, disk2: 0, disk3: 0, disk4: 0, disk5: 0 }
  const disk5Since = addDays(today, -180)
  const dataDisk = (day: string) => {
    const weights: [string, number][] = (
      [
        ['disk1', 1.5],
        ['disk2', 1.3],
        ['disk3', 1.0],
        ['disk4', 0.9],
        ['disk5', day >= disk5Since ? 1.4 : 0],
      ] as const
    ).map(([name, w]) => [name, w * Math.max(0, capacity[name] * 0.93 - filled[name]) ** 2])
    const total = weights.reduce((s, [, w]) => s + w, 0)
    let r = random() * total
    for (const [name, w] of weights) {
      r -= w
      if (r <= 0) return name
    }
    return 'disk4'
  }

  const files: FakeFile[] = []
  let inode = 1
  const add = (path: string, size: number, disk: string, added: string, removed?: string | null): FakeFile => {
    const file: FakeFile = { path, inode: inode++, size: Math.round(size / 4096) * 4096 || 4096, disk, added }
    if (removed) file.removed = removed
    else if (disk in filled) filled[disk] += file.size
    files.push(file)
    return file
  }
  const link = (file: FakeFile, path: string, added: string, removed?: string | null) => {
    const name: FakeFile = { ...file, path, added: added < file.added ? file.added : added }
    if (removed) name.removed = removed
    else delete name.removed
    files.push(name)
  }
  const titles = uniqueNames(random, makeTitle)

  // -- data/media/movies, hardlinked from data/torrents/movies --------------
  for (let i = 0; i < 1080; i++) {
    const title = titles()
    const year = int(1958, 2026)
    const quality = pick([
      'Bluray-2160p',
      'Bluray-2160p',
      'Bluray-1080p',
      'Bluray-1080p',
      'Bluray-1080p',
      'WEBDL-1080p',
    ])
    const size =
      quality === 'Bluray-2160p'
        ? between(38, 82) * GiB
        : quality === 'Bluray-1080p'
          ? between(8, 22) * GiB
          : between(3, 7) * GiB
    const added = someDay(0.62)
    const disk = dataDisk(added)
    const folder = `data/media/movies/${title} (${year})`
    const movie = add(`${folder}/${title} (${year}) ${quality}.mkv`, size, disk, added)
    if (chance(0.7)) add(`${folder}/${title} (${year}) ${quality}.en.srt`, between(40, 110) * KiB, disk, added)
    if (chance(0.66)) {
      const release = `${dots(title)}.${year}.${quality === 'WEBDL-1080p' ? '1080p.WEB-DL' : quality.replace('Bluray-', '') + '.BluRay'}.x265-${pick(groups)}`
      // Seeding ends after a while for some; their media copy stays.
      const stops = chance(0.35) ? dayWithin(added, 240) : null
      link(movie, `data/torrents/movies/${release}/${release}.mkv`, added, stops)
    }
  }
  // Downloads never imported into the library.
  for (let i = 0; i < 46; i++) {
    const added = someDay(0.3)
    const release = `${dots(titles())}.${int(1970, 2026)}.1080p.BluRay.x264-${pick(groups)}`
    add(
      `data/torrents/movies/${release}/${release}.mkv`,
      between(6, 18) * GiB,
      dataDisk(added),
      added,
      chance(0.3) ? dayWithin(added, 200) : null,
    )
  }

  // -- data/media/family (the Movies assignment still points at its old name, kids-movies)
  for (let i = 0; i < 28; i++) {
    const title = titles()
    const year = int(1985, 2025)
    const added = someDay(0.8)
    add(
      `data/media/family/${title} (${year})/${title} (${year}) Bluray-1080p.mkv`,
      between(6, 14) * GiB,
      dataDisk(added),
      added,
    )
  }

  // -- data/media/tv, a season per disk, some seasons hardlinked from torrents
  const shows = uniqueNames(random, makeShow)
  for (let s = 0; s < 92; s++) {
    const show = shows()
    const episodeSize = between(0.9, 4.2) * GiB
    const seasons = int(1, 7)
    let start = someDay(0.55, 1.3)
    for (let season = 1; season <= seasons; season++) {
      const disk = dataDisk(start)
      const sn = String(season).padStart(2, '0')
      const linked = chance(0.4)
      const release = `${dots(show)}.S${sn}.1080p.WEB-DL.x264-${pick(groups)}`
      const stops = linked && chance(0.4) ? dayWithin(start, 300) : null
      const episodes = int(6, 13)
      for (let e = 1; e <= episodes; e++) {
        const added = addDays(start, (e - 1) * 7)
        if (added > today) break
        const en = String(e).padStart(2, '0')
        const name = `${show} - S${sn}E${en} - ${makeEpisode(random)}.mkv`
        const episode = add(
          `data/media/tv/${show}/Season ${sn}/${name}`,
          episodeSize * between(0.75, 1.25),
          disk,
          added,
        )
        if (linked)
          link(episode, `data/torrents/tv/${release}/${dots(show)}.S${sn}E${en}.1080p.WEB-DL.mkv`, added, stops)
      }
      start = addDays(start, int(200, 420))
      if (start > today) break
    }
  }

  // -- data/media/music
  const artists = uniqueNames(random, makeArtist)
  for (let a = 0; a < 46; a++) {
    const artist = artists()
    for (let b = int(1, 5); b > 0; b--) {
      const album = `${makeTitle(random)} (${int(1968, 2026)})`
      const added = someDay(0.8)
      const disk = dataDisk(added)
      const folder = `data/media/music/${artist}/${album}`
      for (let t = int(8, 14); t > 0; t--) {
        add(`${folder}/${String(t).padStart(2, '0')} - ${makeEpisode(random)}.flac`, between(18, 52) * MiB, disk, added)
      }
      add(`${folder}/cover.jpg`, between(0.3, 2.5) * MiB, disk, added)
    }
  }

  // -- data/usenet: downloads pass through and leave within days
  for (let i = 0; i < 160; i++) {
    const added = addDays(firstDay, int(0, historyDays))
    const leaves = addDays(added, int(1, 4))
    const name = `${dots(titles())}.${int(1990, 2026)}.1080p.WEB.h264-${pick(groups)}`
    add(
      `data/usenet/complete/${name}/${name}.mkv`,
      between(2, 14) * GiB,
      dataDisk(added),
      added,
      leaves <= today ? leaves : null,
    )
  }
  add('data/usenet/incomplete/sab_partial_0001.tmp', 3.2 * GiB, 'disk2', addDays(today, -1))

  // -- Photos: one folder a month, phone photos and videos
  const firstYear = Number(today.slice(0, 4)) - 11
  let photoNumber = 1000
  for (let year = firstYear; year <= Number(today.slice(0, 4)); year++) {
    const disk = year % 2 === 0 ? 'disk3' : 'disk4'
    for (let month = 1; month <= 12; month++) {
      const mm = String(month).padStart(2, '0')
      const monthEnd = `${year}-${mm}-28`
      if (monthEnd > today) break
      for (let p = int(10, 46); p > 0; p--) {
        const video = chance(0.1)
        const n = String(photoNumber++).padStart(4, '0')
        add(
          `Photos/${year}/${year}-${mm}/${video ? `VID_${n}.MOV` : `IMG_${n}.HEIC`}`,
          video ? between(40, 620) * MiB : between(1.6, 4.8) * MiB,
          disk,
          monthEnd,
        )
      }
    }
  }
  for (let i = 1; i <= 24; i++)
    add(
      `Photos/Scans/scan-${String(i).padStart(3, '0')}.tiff`,
      between(60, 140) * MiB,
      'disk3',
      addDays(firstDay, -900),
    )

  // -- backups: weekly laptop images kept 8 weeks, monthly flash backups kept forever
  for (let week = Math.floor(-historyDays / 7) - 8; week <= 0; week++) {
    const day = addDays(today, week * 7 - 2)
    const leaves = addDays(day, 56)
    add(`backups/laptop/laptop-${day}.img.zst`, between(96, 128) * GiB, 'disk4', day, leaves <= today ? leaves : null)
  }
  for (let month = -24; month <= 0; month++) {
    const day = addDays(today, month * 30 - 5)
    add(`backups/flash/flash-${day}.zip`, between(480, 720) * MiB, 'disk1', day)
  }

  // -- isos
  for (const [name, gib] of [
    ['ubuntu-26.04-desktop-amd64.iso', 6.1],
    ['debian-14.1.0-amd64-netinst.iso', 0.7],
    ['Win11_25H2_English_x64.iso', 7.4],
    ['virtio-win-0.1.285.iso', 0.7],
    ['memtest86-usb.img', 0.5],
    ['truenas-scale-26.10.iso', 2.1],
  ] as const) {
    add(`isos/${name}`, gib * GiB, 'disk5', someDay(0.4))
  }

  // -- appdata on the cache: many small files that keep growing
  const hex = () => Array.from({ length: 12 }, () => '0123456789abcdef'[int(0, 15)]).join('')
  const growing = (folder: string, count: number, lo: number, hi: number, ext: string, pBefore = 0.5) => {
    for (let i = 0; i < count; i++) add(`${folder}/${hex()}${ext}`, between(lo, hi), 'cache', someDay(pBefore, 1))
  }
  const plex = 'appdata/plex/Library/Application Support/Plex Media Server'
  growing(`${plex}/Metadata/Movies`, 260, 10 * MiB, 90 * MiB, '.bundle')
  growing(`${plex}/Metadata/TV Shows`, 220, 8 * MiB, 70 * MiB, '.bundle')
  growing(`${plex}/Media/localhost`, 180, 20 * MiB, 140 * MiB, '.bif')
  growing(`${plex}/Cache/PhotoTranscoder`, 160, 1 * MiB, 9 * MiB, '.jpg', 0.1)
  add(`${plex}/Plug-in Support/Databases/com.plexapp.plugins.library.db`, 1.4 * GiB, 'cache', addDays(firstDay, -800))
  add(
    `${plex}/Plug-in Support/Databases/com.plexapp.plugins.library.blobs.db`,
    3.1 * GiB,
    'cache',
    addDays(firstDay, -800),
  )
  for (const app of ['sonarr', 'radarr']) {
    add(`appdata/${app}/${app}.db`, between(180, 420) * MiB, 'cache', addDays(firstDay, -700))
    growing(`appdata/${app}/MediaCover`, 140, 0.4 * MiB, 2.2 * MiB, '.jpg')
    growing(`appdata/${app}/logs`, 30, 1 * MiB, 10 * MiB, '.txt', 0)
  }
  growing('appdata/qbittorrent/BT_backup', 120, 20 * KiB, 900 * KiB, '.fastresume')
  add('appdata/home-assistant/home-assistant_v2.db', 2.6 * GiB, 'cache', addDays(firstDay, -400))
  for (let i = 0; i < 6; i++)
    add(`appdata/home-assistant/backups/${hex()}.tar`, between(300, 520) * MiB, 'cache', addDays(today, -i * 7 - 1))
  growing('appdata/immich/thumbs', 320, 2 * MiB, 9 * MiB, '.webp', 0.2)
  growing('appdata/immich/encoded-video', 60, 30 * MiB, 220 * MiB, '.mp4', 0.2)
  growing('appdata/immich-postgres/pgdata/base', 40, 40 * MiB, 160 * MiB, '', 0.3)

  // -- domains (VM disks) and system on the cache
  add('domains/Windows 11/vdisk1.img', 61.4 * GiB, 'cache', addDays(firstDay, -300))
  add('domains/Home Assistant/haos_ova-17.2.qcow2', 17.8 * GiB, 'cache', addDays(firstDay, -500))
  add('domains/ubuntu-t3/vdisk1.img', 93.7 * GiB, 'cache', addDays(today, -95))
  add('system/docker/docker.img', 40 * GiB, 'cache', addDays(firstDay, -900))
  add('system/libvirt/libvirt.img', 1 * GiB, 'cache', addDays(firstDay, -900))

  const xfs = (name: string, size: number) => ({
    name,
    kind: 'data' as const,
    filesystem: 'xfs',
    size,
    overhead: size * 0.0052,
  })
  return {
    firstDay,
    disks: [
      ...Object.entries(capacity).map(([name, size]) => xfs(name, size)),
      { name: 'cache', kind: 'cache', filesystem: 'btrfs', size: 1 * TB, overhead: 7.4 * GiB },
    ],
    parity: [{ name: 'parity', size: 14 * TB, status: 'valid', lastCheck: addDays(today, -12) }],
    files,
    categories: [
      { id: 1, name: 'Movies', colour: '#2a78d6' },
      { id: 2, name: 'TV', colour: '#eb6834' },
      { id: 3, name: 'Photos', colour: '#1baf7a' },
      { id: 4, name: 'Music', colour: '#eda100' },
      { id: 5, name: 'Torrents', colour: '#e87ba4' },
      { id: 6, name: 'Backups', colour: '#008300' },
      { id: 7, name: 'Apps', colour: '#6250d6' },
    ],
    assignments: [
      { path: 'data/media/movies', categoryId: 1 },
      { path: 'data/media/kids-movies', categoryId: 1 },
      { path: 'data/media/tv', categoryId: 2 },
      { path: 'Photos', categoryId: 3 },
      { path: 'data/media/music', categoryId: 4 },
      { path: 'data/torrents', categoryId: 5 },
      { path: 'data/usenet', categoryId: 5 },
      { path: 'backups', categoryId: 6 },
      { path: 'appdata', categoryId: 7 },
      { path: 'domains', categoryId: 7 },
      { path: 'system', categoryId: 7 },
    ],
    scan: { finishedAt: new Date(now.getTime() - 14 * 60_000).toISOString(), durationMs: 3240 },
  }
}

// -- Names (all invented) ------------------------------------------------------

const adjectives = [
  'Silent',
  'Crimson',
  'Hollow',
  'Last',
  'Distant',
  'Broken',
  'Golden',
  'Quiet',
  'Iron',
  'Paper',
  'Velvet',
  'Northern',
  'Salt',
  'Glass',
  'Wild',
  'Lonely',
  'Bright',
  'Burning',
  'Frozen',
  'Hidden',
  'Long',
  'Little',
  'Midnight',
  'Painted',
  'Restless',
  'Scarlet',
  'Secret',
  'Shallow',
  'Small',
  'Sudden',
  'Tender',
  'Violet',
  'Wandering',
  'Winter',
  'Amber',
  'Copper',
  'Electric',
  'Gentle',
  'Kind',
  'Open',
]
const nouns = [
  'Harbor',
  'Orchard',
  'Signal',
  'River',
  'Garden',
  'Engine',
  'Lantern',
  'Meridian',
  'Tide',
  'Horizon',
  'Archive',
  'Canyon',
  'Comet',
  'Country',
  'Echo',
  'Field',
  'Fortune',
  'Frontier',
  'Island',
  'Kingdom',
  'Ladder',
  'Machine',
  'Mirror',
  'Mountain',
  'Ocean',
  'Orbit',
  'Passage',
  'Promise',
  'Quarry',
  'Road',
  'Season',
  'Shadow',
  'Station',
  'Summer',
  'Thread',
  'Tower',
  'Valley',
  'Voyage',
  'Window',
  'Wolf',
]
const places = [
  'Brightwater',
  'Hollowmere',
  'Kestrel Bay',
  'Ashford',
  'Marrow Hill',
  'Tamsin Row',
  'Port Ellery',
  'Gray Hollow',
  'Linden Park',
  'Saltmarsh',
]
const people = [
  'Ada',
  'Bram',
  'Cleo',
  'Dorian',
  'Edie',
  'Felix',
  'Greta',
  'Hugo',
  'Iris',
  'Jonah',
  'Kit',
  'Lena',
  'Milo',
  'Nell',
  'Otto',
  'Pia',
]
const groups = ['NOVA', 'KESTREL', 'HALCYON', 'TIDEPOOL', 'LUMEN', 'QUARTZ', 'VESPER', 'ORCHID']

function makeTitle(random: () => number): string {
  const p = <T>(items: readonly T[]) => items[Math.floor(random() * items.length)]
  switch (Math.floor(random() * 6)) {
    case 0:
      return `The ${p(adjectives)} ${p(nouns)}`
    case 1:
      return `${p(nouns)} of the ${p(adjectives)} ${p(nouns)}`
    case 2:
      return `${p(adjectives)} ${p(nouns)}`
    case 3:
      return `The ${p(nouns)} at ${p(places)}`
    case 4:
      return `${p(people)} and the ${p(nouns)}`
    default:
      return `${p(nouns)}s`
  }
}

function makeShow(random: () => number): string {
  const p = <T>(items: readonly T[]) => items[Math.floor(random() * items.length)]
  switch (Math.floor(random() * 4)) {
    case 0:
      return p(places)
    case 1:
      return `${p(adjectives)} ${p(nouns)}`
    case 2:
      return `The ${p(nouns)}`
    default:
      return `${p(people)}'s ${p(nouns)}`
  }
}

function makeArtist(random: () => number): string {
  const p = <T>(items: readonly T[]) => items[Math.floor(random() * items.length)]
  return random() < 0.5 ? `The ${p(adjectives)} ${p(nouns)}s` : `${p(people)} ${p(nouns)}`
}

function makeEpisode(random: () => number): string {
  const p = <T>(items: readonly T[]) => items[Math.floor(random() * items.length)]
  return random() < 0.5 ? `The ${p(nouns)}` : `${p(adjectives)} ${p(nouns)}`
}

function dots(name: string): string {
  return name.replace(/[^A-Za-z0-9]+/g, '.').replace(/^\.|\.$/g, '')
}

/** A source of names that never repeats one (it adds a number when it runs dry). */
function uniqueNames(random: () => number, make: (random: () => number) => string): () => string {
  const seen = new Set<string>()
  return () => {
    let name = make(random)
    for (let n = 2; seen.has(name); n++) name = n > 6 ? `${make(random)} ${n}` : make(random)
    seen.add(name)
    return name
  }
}

/** A small seeded random number generator: the same seed, the same server. */
function mulberry32(seed: number): () => number {
  let a = seed >>> 0
  return () => {
    a = (a + 0x6d2b79f5) >>> 0
    let t = a
    t = Math.imul(t ^ (t >>> 15), t | 1)
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61)
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296
  }
}
