const fs = require('fs');
const path = 'frontend/app/(dashboard)/transactions/page.tsx';
let code = fs.readFileSync(path, 'utf8');

// Remove handleReview from its current location
const handleReviewTarget = `  const handleReview = useCallback((id: string, categoryId: string) => {
    reviewMutation.mutate({ id, categoryId });
  }, [reviewMutation]);

`;

code = code.replace(handleReviewTarget, '');

// Insert it after reviewMutation
const reviewMutationTarget = `      toast.error(err.message || "Failed to categorize transaction");
    },
  });`;

const reviewMutationReplacement = `      toast.error(err.message || "Failed to categorize transaction");
    },
  });

  const handleReview = useCallback((id: string, categoryId: string) => {
    reviewMutation.mutate({ id, categoryId });
  }, [reviewMutation]);`;

code = code.replace(reviewMutationTarget, reviewMutationReplacement);

fs.writeFileSync(path, code);
console.log("Moved handleReview below reviewMutation!");
