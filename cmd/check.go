package cmd

import (
	"fmt"
	"os"

	"cosmossdk.io/log"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	packetforwardtypes "github.com/cosmos/ibc-apps/middleware/packet-forward-middleware/v8/packetforward/types"
	icqtypes "github.com/cosmos/ibc-apps/modules/async-icq/v8/types"
	icahosttypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/host/types"
	ibctransfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"
	ibchost "github.com/cosmos/ibc-go/v8/modules/core/exported"

	"cosmossdk.io/store/iavl"
	"cosmossdk.io/store/metrics"
	"cosmossdk.io/store/types"
	storetypes "cosmossdk.io/store/types"
	evidencetypes "cosmossdk.io/x/evidence/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	dbm "github.com/cosmos/cosmos-db"
	consensusparamtypes "github.com/cosmos/cosmos-sdk/x/consensus/types"
	crisistypes "github.com/cosmos/cosmos-sdk/x/crisis/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/spf13/cobra"
	"github.com/syndtr/goleveldb/leveldb/opt"

	"cosmossdk.io/store/rootmulti"
)

func checkStoreVersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check-store-versions [path_to_home]",
		Short: "check versions available in all stores",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return checkStoreVersions(args[0])
		},
	}
	return cmd
}

func checkStoreVersions(home string) error {
	dbDir := rootify(dataDir, home)

	o := opt.Options{
		DisableSeeksCompaction: true,
	}

	appDB, err := dbm.NewGoLevelDBWithOpts("application", dbDir, &o)
	if err != nil {
		return err
	}

	fmt.Println("checking store versions")

	// only mount keys from core sdk
	keys := storetypes.NewKVStoreKeys(
		authtypes.StoreKey, banktypes.StoreKey, authzkeeper.StoreKey, stakingtypes.StoreKey, distrtypes.StoreKey, slashingtypes.StoreKey, ibchost.StoreKey,
		icahosttypes.StoreKey,
		icqtypes.StoreKey,
		evidencetypes.StoreKey, minttypes.StoreKey, govtypes.StoreKey, ibctransfertypes.StoreKey,
		packetforwardtypes.StoreKey,
		paramstypes.StoreKey, consensusparamtypes.StoreKey, crisistypes.StoreKey, upgradetypes.StoreKey,
	)

	if app == "osmosis" {
		osmoKeys := storetypes.NewKVStoreKeys(
			"downtimedetector",
			"hooks-for-ibc",
			"lockup",
			"concentratedliquidity",
			"gamm",
			"cosmwasmpool",
			"poolmanager",
			"twap",
			"epochs",
			"protorev",
			"txfees",
			"incentives",
			"poolincentives",
			"tokenfactory",
			"valsetpref",
			"superfluid",
			"wasm",
			"smartaccount",
		)
		for key, value := range osmoKeys {
			keys[key] = value
		}
	}

	logger := log.NewLogger(os.Stderr)
	appStore := rootmulti.NewStore(appDB, logger, metrics.NewMetrics([][]string{}))

	for _, value := range keys {
		appStore.MountStoreWithDB(value, storetypes.StoreTypeIAVL, nil)
		appStore.SetIAVLDisableFastNode(true)
	}

	err = appStore.LoadLatestVersion()
	if err != nil {
		return err
	}

	storeKeysByName := appStore.StoreKeysByName()

	for storeName, storeKey := range storeKeysByName {
		store := appStore.GetCommitKVStore(storeKey)

		if store.GetStoreType() != types.StoreTypeIAVL {
			continue
		}

		versions := store.(*iavl.Store).GetAllVersions()
		versionExists := store.(*iavl.Store).VersionExists(int64(versions[0]))
		fmt.Println("key", storeName, "versions available", len(versions), "first version", versions[0], "exists", versionExists, "last version", versions[len(versions)-1])
	}

	return nil
}
