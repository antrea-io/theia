.PHONY: all
all: policy_recommendation.zip traffic_drop.zip

policy_recommendation.zip: policy_recommendation/*.py
	@zip $@ $^

traffic_drop.zip: traffic_drop/*.py
	@zip $@ $^

.PHONY: clean
clean:
	rm -f *.zip

.PHONY: check
check:
	PYTHONPATH="${PYTHONPATH}:$(CURDIR)" python3 -m unittest discover policy_recommendation "*_test.py"
	PYTHONPATH="${PYTHONPATH}:$(CURDIR)" python3 -m unittest discover traffic_drop "*_test.py"
