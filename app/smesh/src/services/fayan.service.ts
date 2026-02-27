import DataLoader from 'dataloader'

class FayanService {
  static instance: FayanService

  private userPercentileDataLoader = new DataLoader<string, number | null>(
    async (pubkeys) => {
      try {
        const res = await fetch(`https://fayan.smesh.social/users`, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json'
          },
          body: JSON.stringify({ pubkeys })
        })
        if (!res.ok) {
          return new Array(pubkeys.length).fill(null)
        }
        const data = await res.json()
        return pubkeys.map((pubkey) => data[pubkey] ?? 0)
      } catch {
        return new Array(pubkeys.length).fill(null)
      }
    },
    { maxBatchSize: 50 }
  )

  constructor() {
    if (!FayanService.instance) {
      FayanService.instance = this
    }
    return FayanService.instance
  }

  // null means server error
  async fetchUserPercentile(userId: string): Promise<number | null> {
    return await this.userPercentileDataLoader.load(userId)
  }
}

const instance = new FayanService()
export default instance
