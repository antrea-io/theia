.PHONY: all
all: policy_recommendation.zip

policy_recommendation.zip: policy_recommendation/*.py
	@zip $@ $^

.PHONY: clean
clean:
	rm -f *.zip

.PHONY: check
check:
	PYTHONPATH="${PYTHONPATH}:$(CURDIR)" python3 -m unittest discover policy_recommendation "*_test.py"
