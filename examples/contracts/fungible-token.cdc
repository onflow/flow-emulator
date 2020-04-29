// The main Fungible Token interface.
// Other fungible token contracts will implement this interface
//
pub contract interface FungibleToken {

    // The total number of tokens in existence.
    // It is up to the implementer to ensure that total supply
    // stays accurate and up to date
    pub var totalSupply: UFix64

    // Event that is emitted when the contract is created
    pub event FungibleTokenInitialized(initialSupply: UFix64)

    // Event that is emitted when tokens are withdrawn from a Vault
    pub event Withdraw(amount: UFix64, from: Address?)

    // Event that is emitted when tokens are deposited to a Vault
    pub event Deposit(amount: UFix64, to: Address?)


    pub resource interface Provider {

        pub fun withdraw(amount: UFix64): @Vault {
            post {
                // `result` refers to the return value
                result.balance == amount:
                    "Withdrawal amount must be the same as the balance of the withdrawn Vault"
            }
        }
    }

    pub resource interface Receiver {

        // deposit
        //
        // Function that can be called to deposit tokens
        // into the implementing resource type
        //
        pub fun deposit(from: @Vault) {
            pre {
                from.balance > UFix64(0):
                    "Deposit balance must be positive"
            }
        }
    }

    pub resource interface Balance {

        // The total balance of the account's tokens
        pub var balance: UFix64

        init(balance: UFix64) {
            post {
                self.balance == balance:
                    "Balance must be initialized to the initial balance"
            }
        }
    }

    pub resource Vault: Provider, Receiver, Balance {
        // The total balance of the accounts tokens
        pub var balance: UFix64

        // The conforming type must declare an initializer
        // that allows prioviding the initial balance of the vault
        //
        init(balance: UFix64)

        // withdraw subtracts `amount` from the vaults balance and
        // returns a vault object with the subtracted balance
        //
        pub fun withdraw(amount: UFix64): @Vault {
            pre {
                self.balance >= amount:
                    "Amount withdrawn must be less than or equal than the balance of the Vault"
            }
            post {
                // use the special function `before` to get the value of the `balance` field
                // at the beginning of the function execution
                //
                self.balance == before(self.balance) - amount:
                    "New Vault balance must be the difference of the previous balance and the withdrawn Vault"
            }
        }

        // deposit takes a vault object as a parameter and adds
        // its balance to the balance of the stored vault
        //
        pub fun deposit(from: @Vault) {
            post {
                self.balance == before(self.balance) + before(from.balance):
                    "New Vault balance must be the sum of the previous balance and the deposited Vault"
            }
        }
    }

    pub fun createEmptyVault(): @Vault {
        post {
            result.balance == UFix64(0): "The newly created Vault must have zero balance"
        }
    }
}

pub contract FlowToken: FungibleToken {

    // Total supply of flow tokens in existence
    pub var totalSupply: UFix64

    // Event that is emitted when the contract is created
    pub event FungibleTokenInitialized(initialSupply: UFix64)

    // Event that is emitted when tokens are withdrawn from a Vault
    pub event Withdraw(amount: UFix64, from: Address?)

    // Event that is emitted when tokens are deposited to a Vault
    pub event Deposit(amount: UFix64, to: Address?)

    // Event that is emitted when new tokens are minted
    pub event Mint(amount: UFix64)

    // Event that is emitted when tokens are destroyed
    pub event Burn(amount: UFix64)

    // Event that is emitted when a mew minter resource is created
    pub event MinterCreated(allowedAmount: UFix64)

    pub resource Vault: FungibleToken.Provider, FungibleToken.Receiver, FungibleToken.Balance {

        // holds the balance of a users tokens
        pub var balance: UFix64

        // initialize the balance at resource creation time
        init(balance: UFix64) {
            self.balance = balance
        }

        pub fun withdraw(amount: UFix64): @FlowToken.Vault {
            self.balance = self.balance - amount
            emit Withdraw(amount: amount, from: self.owner?.address)
            return <-create Vault(balance: amount)
        }

        pub fun deposit(from: @Vault) {
            let vault <- from as! @FlowToken.Vault
            self.balance = self.balance + vault.balance
            emit Deposit(amount: vault.balance, to: self.owner?.address)
            vault.balance = 0.0
            destroy vault
        }

        destroy() {
            FlowToken.totalSupply = FlowToken.totalSupply - self.balance
        }
    }

    pub fun createEmptyVault(): @FlowToken.Vault {
        return <-create Vault(balance: 0.0)
    }

    pub resource MintAndBurn {

        // the amount of tokens that the minter is allowed to mint
        pub var allowedAmount: UFix64

        pub fun mintTokens(amount: UFix64): @FlowToken.Vault {
            pre {
                amount > UFix64(0): "Amount minted must be greater than zero"
                amount <= self.allowedAmount: "Amount minted must be less than the allowed amount"
            }
            FlowToken.totalSupply = FlowToken.totalSupply + amount
            self.allowedAmount = self.allowedAmount - amount
            emit Mint(amount: amount)
            return <-create FlowToken.Vault(balance: amount)
        }

        pub fun burnTokens(from: @Vault) {
            let vault <- from as! @FlowToken.Vault
            let amount = vault.balance
            destroy vault
            emit Burn(amount: amount)
        }

        pub fun createNewMinter(allowedAmount: UFix64): @MintAndBurn {
            emit MinterCreated(allowedAmount: allowedAmount)
            return <-create MintAndBurn(allowedAmount: allowedAmount)
        }

        init(allowedAmount: UFix64) {
            self.allowedAmount = allowedAmount
        }
    }

    init() {
        // Initialize the totalSupply field to the initial balance
        self.totalSupply = 1000.0

        // Create the Vault with the total supply of tokens and save it in storage
        //
        let vault <- create Vault(balance: self.totalSupply)
        self.account.save(<-vault, to: /storage/flowTokenVault)

        // Create a public capability to the stored Vault that only exposes
        // the `deposit` method through the `Receiver` interface
        //
        self.account.link<&{FungibleToken.Receiver}>(
            /public/flowTokenReceiver,
            target: /storage/flowTokenVault
        )

        // Create a public capability to the stored Vault that only exposes
        // the `balance` field through the `Balance` interface
        //
        self.account.link<&{FungibleToken.Balance}>(
            /public/flowTokenBalance,
            target: /storage/flowTokenVault
        )

        // Create a new MintAndBurn resource and store it in account storage
        let mintAndBurn <- create MintAndBurn(allowedAmount: 100.0)
        self.account.save(<-mintAndBurn, to: /storage/flowTokenMintAndBurn)

        // Emit an event that shows that the contract was initialized
        emit FungibleTokenInitialized(initialSupply: self.totalSupply)
    }
}

 
 