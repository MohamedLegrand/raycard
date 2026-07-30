package commun

// RegleKyc représente les plafonds réglementaires applicables à un
// palier KYC dans un pays donné. Ces règles sont lues depuis la table
// regles_kyc_pays — jamais codées en dur — car elles varient d'un pays
// à l'autre (exigence multi-pays dès la V1).
type RegleKyc struct {
	PaysCode               string
	Tier                   KycTier
	Devise                 string // ISO 4217
	PlafondSoldeCentimes   int64
	PlafondMensuelCentimes int64
}
