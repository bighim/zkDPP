# zkDPP Product, State, Process 최소 설계

이 문서는 여러 입력 원자재가 하나의 공정을 거쳐 주제품과 폐기물로 나뉘는 과정을 설명합니다. 숫자 예시를 먼저 살펴본 뒤, 같은 구조를 일반화합니다.

## 1. 한눈에 보는 큰 그림

Process는 다음 두 단계로 구성됩니다.

```text
입력 원자재 합산
  → 공정에서 발생한 변화 반영
  → 주제품과 폐기물로 분배
```

Circuit Setup과 proof 검증은 다음 순서로 진행됩니다.

```text
Factory가 공정 규칙과 근거 문서 제출
  → Certificator가 검토하고 Circuit Setup
  → Factory가 ProvingKey로 proof 생성
  → Contract가 VerifyingKey로 proof 검증
```

입력 원자재는 private witness이므로 외부에 드러나지 않습니다. 출력 주제품과 폐기물에 대한 설명은 외부에 드러날 수 있습니다.

Circuit은 Product Type을 검사하지 않습니다. Circuit이 검사하는 값은 전체 질량, 탄소발자국, 재활용 원료 질량입니다.

## 2. 숫자로 이해하는 3-to-2 Process

### 2.1 Product와 State

실제 Product는 다음과 같이 구성됩니다.

$$
\mathrm{Product}=(\mathrm{MetadataHash},\mathrm{MergeProfile},T,(C,R))
$$

Process의 수치 계산만 설명할 때는 다음처럼 간단히 표현합니다.

$$
P=(T,C,R)
$$

| 기호 | 의미 | 단위 |
|---|---|---|
| $T$ | Product의 전체 질량입니다. | kg |
| $C$ | Product에 누적된 탄소발자국입니다. | kgCO2e |
| $R$ | Product에 포함된 재활용 원료의 질량입니다. | kg |

재활용 함량은 별도로 저장하지 않고 $R/T$로 계산합니다.

### 2.2 입력 원자재 합산

입력 원자재 세 개를 다음과 같이 가정합니다.

| 입력 | 질량 $T$ | 탄소 $C$ | 재활용 원료 $R$ |
|---|---:|---:|---:|
| 원자재 $A$ | 100 | 80 | 20 |
| 원자재 $B$ | 70 | 120 | 10 |
| 원자재 $C$ | 150 | 30 | 0 |
| **입력 합계** | **320** | **230** | **30** |

입력 합계를 $S_{\mathrm{in}}$이라고 부릅니다.

$$
S_{\mathrm{in}}=A+B+C=(320,230,30)
$$

### 2.3 공정 변화를 표현하는 두 가지 방법

같은 공정 변화를 **절대 변화량** 또는 **상대 변화율**로 표현할 수 있습니다. 두 방식은 이 숫자 예시에서는 같은 결과를 만들지만, 일반적인 공정 규칙으로서는 서로 다른 의미를 가집니다.

#### 방법 1. 절대 변화량

질량이 20 kg 감소하고 탄소발자국이 30 kgCO2e 증가하며 재활용 원료는 변하지 않는다고 표현합니다.

$$
\Delta=(-20,30,0)
$$

공정 후 합계를 $S_{\mathrm{after}}$라고 부르면 다음과 같습니다.

$$
S_{\mathrm{after}}=S_{\mathrm{in}}+\Delta=(300,260,30)
$$

이 방식은 공정의 **절대 손실량과 절대 추가량**을 나타냅니다. 입력 규모가 달라도 항상 질량 20 kg이 감소하고 탄소 30 kgCO2e가 추가되는 규칙입니다.

#### 방법 2. 상대 변화율

입력 합계와 공정 후 합계의 비율을 다음과 같이 표현할 수도 있습니다.

$$
q=\left(\frac{15}{16},\frac{26}{23},1\right)
$$

각 입력 값에 같은 위치의 비율을 곱합니다.

$$
S_{\mathrm{after}}
=
\left(
320\times\frac{15}{16},
230\times\frac{26}{23},
30\times1
\right)
=(300,260,30)
$$

이 방식은 공정의 **수율과 증가율**을 나타냅니다. 입력 규모가 달라지면 손실량과 추가량도 같은 비율로 달라집니다.

#### 두 방식의 정보 노출 차이

| 전달 정보 | 알 수 있는 내용 | 알 수 없는 내용 |
|---|---|---|
| 실제 $S_{\mathrm{in}}$ 또는 $S_{\mathrm{after}}$ | 실제 생산 규모와 State 값 | 숨겨지는 값이 거의 없습니다. |
| 절대 변화량 $\Delta$ | 질량 손실량과 탄소 추가량 | 입력과 출력의 실제 State |
| 상대 변화율 $q$ | 질량 수율과 탄소 증가율 | 입력과 출력의 실제 규모 |

Factory는 실제 batch의 $S_{\mathrm{in}}$과 $S_{\mathrm{after}}$를 Circuit Setup 정보로 제출하지 않습니다. 실제 State는 proof를 만들 때 사용하는 private witness입니다.

Factory는 Setup하려는 Circuit의 의미에 따라 $\Delta$ 또는 $q$ 중 하나를 Certificator에게 제안할 수 있습니다. Certificator는 근거 문서를 검토하고 선택된 규칙을 강제하는 Circuit을 Setup합니다.

다만 실제 출력 State가 별도로 공개되면 $\Delta$나 $q$를 이용해 입력 합계를 역산할 수 있습니다. 따라서 실제 State의 공개 여부와 공정 규칙의 공개 여부는 함께 결정해야 합니다.

### 2.4 주제품과 폐기물로 분배

공정 후 합계 $S_{\mathrm{after}}$를 주제품 $D$와 폐기물 $W$로 분배합니다.

| 출력 | 질량 $T$ | 탄소 $C$ | 재활용 원료 $R$ |
|---|---:|---:|---:|
| 주제품 $D$ | 270 | 260 | 30 |
| 폐기물 $W$ | 30 | 0 | 0 |

이 예시에서는 질량의 90%가 주제품으로 가고 10%가 폐기물로 갑니다. 탄소와 재활용 원료는 모두 주제품에 배분합니다.

주제품 분배를 행렬로 펼쳐 쓰면 다음과 같습니다.

$$
\begin{pmatrix}
D_T\\
D_C\\
D_R
\end{pmatrix}
=
\begin{pmatrix}
\frac{9}{10} & 0 & 0\\
0 & 1 & 0\\
0 & 0 & 1
\end{pmatrix}
\begin{pmatrix}
S_{\mathrm{after},T}\\
S_{\mathrm{after},C}\\
S_{\mathrm{after},R}
\end{pmatrix}
$$

폐기물 분배는 다음과 같습니다.

$$
\begin{pmatrix}
W_T\\
W_C\\
W_R
\end{pmatrix}
=
\begin{pmatrix}
\frac{1}{10} & 0 & 0\\
0 & 0 & 0\\
0 & 0 & 0
\end{pmatrix}
\begin{pmatrix}
S_{\mathrm{after},T}\\
S_{\mathrm{after},C}\\
S_{\mathrm{after},R}
\end{pmatrix}
$$

따라서 다음 결과가 나옵니다.

$$
D=(270,260,30),\qquad W=(30,0,0)
$$

$$
D+W=S_{\mathrm{after}}
$$

행렬 안의 0은 질량, 탄소, 재활용 원료를 서로 섞지 않는다는 뜻입니다. 예를 들어 질량 값이 탄소 값으로 변환되지는 않습니다.

이 문서에서 행렬은 분배 규칙을 사람이 직관적으로 읽기 위한 표현입니다. Circuit 성능이나 행렬곱 구현 방식은 이 문서의 범위가 아닙니다.

#### 표준과 방법론에서의 폐기물 구분

[GHG Protocol Product Life Cycle Standard](https://ghgprotocol.org/sites/default/files/ghgp/standards/Product-Life-Cycle-Accounting-Reporting-Standard-EReader_041613_0.pdf)는 출력이 실제 폐기물이라면 주제품과 폐기물 사이의 탄소 배분이 필요하지 않고, 탄소를 주제품에 귀속하며 폐기물 처리를 별도 공정으로 포함하는 방식을 설명합니다. 반대로 해당 출력이 이후 사용되거나 경제적 가치를 가지면 더 이상 단순 폐기물로 보지 않고 공동제품에 맞는 배분 방법을 적용해야 합니다.

EU의 [Environmental Footprint 방법](https://environment.ec.europa.eu/system/files/2021-12/Annexes%201%20to%202.pdf)도 제품 전 생애주기의 투입·산출 흐름과 환경 영향을 일관된 방법으로 모델링하도록 합니다.

이 자료들은 모든 공정에서 반드시 폐기물 출력을 만들도록 요구하는 일반 법률은 아닙니다. 다만 공정에서 실제 폐기물이 발생한다면 이를 주제품과 구분하고, 어떤 배분 방법을 사용했는지 설명할 필요가 있다는 근거를 제공합니다.

## 3. 예시의 일반화

입력 Product가 $n$개이고 출력 Product가 $m$개인 Process도 같은 순서로 표현합니다.

입력 Product를 합산합니다.

$$
S_{\mathrm{in}}=\sum_{i=1}^{n}X_i
$$

공정 변화는 절대 변화량 방식 또는 상대 변화율 방식 중 하나로 표현할 수 있습니다.

$$
S_{\mathrm{after}}=S_{\mathrm{in}}+\Delta
$$

또는 각 위치에 같은 위치의 변화율을 곱합니다.

$$
S_{\mathrm{after}}
=
\left(
q_T S_{\mathrm{in},T},
q_C S_{\mathrm{in},C},
q_R S_{\mathrm{in},R}
\right)
$$

현재 모델에서는 Process가 재활용 원료를 새로 만들거나 제거하지 않으므로 다음 조건을 사용합니다.

$$
\Delta_R=0
\qquad\text{또는}\qquad
q_R=1
$$

출력 $Y_j$는 Certificator가 승인한 분배 행렬 $M_j$에 따라 계산합니다.

$$
Y_j=M_jS_{\mathrm{after}}
$$

모든 출력의 합은 공정 후 합계와 일치해야 합니다.

$$
\sum_{j=1}^{m}Y_j=S_{\mathrm{after}}
$$

## 4. Factory와 Certificator의 역할

Factory가 Certificator에게 제출하는 최소 정보는 다음과 같습니다.

| Factory가 제출하는 내용 | 처리 방식 |
|---|---|
| 입력 개수와 출력 개수 | Circuit에서 강제합니다. |
| 절대 변화량 $\Delta$ 또는 상대 변화율 $q$ | 선택한 방식에 따라 Circuit에서 강제합니다. |
| 출력별 State 분배 행렬 | Circuit에서 강제합니다. |
| 출력 주제품과 폐기물 설명 | Certificator가 외부에서 검토합니다. |
| 변화와 분배를 뒷받침하는 문서 | Certificator가 외부에서 검토합니다. |

Certificator는 Factory가 제안한 $\Delta$ 또는 $q$와 분배 행렬이 근거 문서에 부합하는지 검토합니다. 이후 선택된 Numeric State 규칙을 검사하는 Circuit을 Setup합니다.

Factory는 Setup 결과로 받은 `ProvingKey`를 사용해 실제 입력과 출력이 승인된 규칙을 따랐다는 proof를 생성합니다. 온체인 Contract는 등록된 `VerifyingKey`로 proof를 검증합니다.

Circuit은 Product Type이나 근거 문서의 진실성을 판단하지 않습니다. Product와 폐기물의 의미 및 근거 자료는 Certificator가 검토하고, Circuit은 승인된 Numeric State 관계만 강제합니다.
