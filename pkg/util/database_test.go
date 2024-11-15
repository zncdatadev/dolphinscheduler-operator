package util_test

// Extracts database info from a valid connection string
import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/zncdatadev/dolphinscheduler-operator/pkg/util"
	"github.com/zncdatadev/operator-go/pkg/client"
)

var _ = Describe("DataBaseExtractor", func() {
	var cli *client.Client
	BeforeEach(func() {
		cli = client.NewClient(k8sClient, nil)
	})

	Context("when extracting database info", func() {
		It("should extract database info from a valid connection string", func() {
			// given
			connectionString := "jdbc:postgresql://127.0.0.1:5432/dolphinscheduler?user=root&password=root"
			d := util.NewDataBaseExtractor(cli, &connectionString)

			// when
			dbInfo, err := d.ExtractDatabaseInfo(context.Background())

			// then
			Expect(err).ToNot(HaveOccurred())
			Expect(dbInfo).NotTo(BeNil())
			Expect(dbInfo.DbType).To(Equal("postgresql"))
			Expect(dbInfo.Driver).To(Equal("org.postgresql.Driver"))
			Expect(dbInfo.Host).To(Equal("127.0.0.1"))
			Expect(dbInfo.Port).To(Equal("5432"))
			Expect(dbInfo.DbName).To(Equal("dolphinscheduler"))
			Expect(dbInfo.Username).To(Equal("root"))
			Expect(dbInfo.Password).To(Equal("root"))
		})

		It("should return error when connection string is nil", func() {
			// given
			d := &util.DataBaseExtractor{
				ConnectionString: nil,
			}

			// when
			dbInfo, err := d.ExtractDatabaseInfo(context.Background())

			// then
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("connection string is empty"))
			Expect(dbInfo).To(BeNil())
		})
	})
})
